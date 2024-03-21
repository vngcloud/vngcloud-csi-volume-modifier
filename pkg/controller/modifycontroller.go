package controller

import (
	"context"
	"fmt"
	"k8s.io/apimachinery/pkg/types"
	"strings"
	"sync"
	"time"

	lapiCoreV1 "k8s.io/api/core/v1"
	lmetaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	lcoreV1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	"github.com/vngcloud/vngcloud-csi-volume-modifier/pkg/modifier"
	lutil "github.com/vngcloud/vngcloud-csi-volume-modifier/pkg/util"
)

// _____________________________________________________________________________________________________________________PUBLIC METHODS

func NewModifyController(
	pname string,
	pmodifier modifier.IModifier,
	pkubeClient kubernetes.Interface,
	presyncPeriod time.Duration,
	pinformerFactory informers.SharedInformerFactory,
	ppvcRateLimiter workqueue.RateLimiter,
	pretryModificationFailures bool,
) IModifyController {
	pvInformer := pinformerFactory.Core().V1().PersistentVolumes()
	pvcInformer := pinformerFactory.Core().V1().PersistentVolumeClaims()
	claimQueue := workqueue.NewNamedRateLimitingQueue(ppvcRateLimiter, fmt.Sprintf("%s-modify-pvc", pname))

	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(&lcoreV1.EventSinkImpl{Interface: pkubeClient.CoreV1().Events(lapiCoreV1.NamespaceAll)})
	eventRecorder := eventBroadcaster.NewRecorder(scheme.Scheme, lapiCoreV1.EventSource{Component: fmt.Sprintf("volume-modifier-for-k8s-%s", pname)})

	ctrl := &modifyController{
		name:                   pname,
		annPrefix:              fmt.Sprintf(AnnotationPrefixPattern, pname),
		modifier:               pmodifier,
		kubeClient:             pkubeClient,
		claimQueue:             claimQueue,
		pvSynced:               pvInformer.Informer().HasSynced,
		pvcSynced:              pvcInformer.Informer().HasSynced,
		volumes:                pvInformer.Informer().GetStore(),
		claims:                 pvcInformer.Informer().GetStore(),
		eventRecorder:          eventRecorder,
		modificationInProgress: make(map[string]struct{}),
	}

	pvcInformer.Informer().AddEventHandlerWithResyncPeriod(cache.ResourceEventHandlerFuncs{
		AddFunc:    ctrl.addPVC,
		UpdateFunc: ctrl.updatePVC,
		DeleteFunc: ctrl.deletePVC,
	}, presyncPeriod)

	return ctrl
}

// _____________________________________________________________________________________________________________________ENTITIES

type modifyController struct {
	name          string
	annPrefix     string
	modifier      modifier.IModifier
	kubeClient    kubernetes.Interface
	claimQueue    workqueue.RateLimitingInterface
	eventRecorder record.EventRecorder
	pvSynced      cache.InformerSynced
	pvcSynced     cache.InformerSynced

	modificationInProgress   map[string]struct{}
	modificationInProgressMu sync.Mutex

	volumes cache.Store
	claims  cache.Store

	retryFailures bool
}

func (s *modifyController) Run(workers int, ctx context.Context) {
	defer s.claimQueue.ShutDown()

	klog.InfoS("Starting external modifier", "name", s.name)
	defer klog.InfoS("Shutting down external modifier", "name", s.name)

	stopCh := ctx.Done()
	informersSyncd := []cache.InformerSynced{s.pvSynced, s.pvcSynced}

	if !cache.WaitForCacheSync(stopCh, informersSyncd...) {
		klog.Errorf("Cannot sync pv or pvc caches")
		return
	}

	for i := 0; i < workers; i++ {
		go wait.Until(s.syncPVCs, 0, stopCh)
	}

	<-stopCh
}

func (s *modifyController) addPVC(obj interface{}) {
	objKey, err := getObjectKeys(obj)
	if err != nil {
		klog.ErrorS(err, "unable to add obj to claim queue")
		return
	}
	s.claimQueue.Add(objKey)
}

func (s *modifyController) updatePVC(old, new interface{}) {
	klog.V(6).InfoS("Received update from shared informer", "old", old, "new", new)

	oldPvc, ok := old.(*lapiCoreV1.PersistentVolumeClaim)
	if !ok || oldPvc == nil {
		return
	}

	newPvc, ok := new.(*lapiCoreV1.PersistentVolumeClaim)
	if !ok || newPvc == nil {
		return
	}

	if s.needsProcessing(oldPvc, newPvc) {
		s.addPVC(new)
	}
}

func (s *modifyController) deletePVC(pobj interface{}) {
	klog.V(6).InfoS("Received delete from shared informer", "obj", pobj)
	objKey, err := getObjectKeys(pobj)
	if err != nil {
		return
	}
	s.claimQueue.Forget(objKey)
}

// Checks if a PVC needs to be processed after an Update.
// Gets a list of all annotations beginning with "<driver-name>/" both PVCs.
// Then checks if the annotations are different between the old and new PVCs.
// If any of them are, this PVC needs to be processed.
func (s *modifyController) needsProcessing(pold *lapiCoreV1.PersistentVolumeClaim, pnew *lapiCoreV1.PersistentVolumeClaim) bool {
	if pold.ResourceVersion == pnew.ResourceVersion {
		return false
	}

	annotations := make(map[string]struct{})
	for key, _ := range pnew.Annotations {
		if s.isValidAnnotation(key) {
			annotations[key] = struct{}{}
		}
	}

	for a, _ := range annotations {
		oldValue := pold.Annotations[a]
		newValue := pnew.Annotations[a]

		if oldValue != newValue {
			return true
		}
	}

	hasBeenBound := pold.Status.Phase != pnew.Status.Phase && pnew.Status.Phase == lapiCoreV1.ClaimBound
	// If the annotation was set at creation we might have skipped the PVC because it was not bound yet
	return len(annotations) > 0 && hasBeenBound
}

func (s *modifyController) isValidAnnotation(pann string) bool {
	return strings.HasPrefix(pann, fmt.Sprintf(AnnotationPrefixPattern, s.name)) &&
		!strings.HasSuffix(pann, "-status")
}

func (s *modifyController) syncPVCs() {
	key, quit := s.claimQueue.Get()
	if quit {
		return
	}
	defer s.claimQueue.Done(key)

	if err := s.syncPVC(key.(string)); err != nil {
		klog.ErrorS(err, "error syncing PVC", "key", key)
		if s.retryFailures {
			s.claimQueue.AddRateLimited(key)
		}
	} else {
		s.claimQueue.Forget(key)
	}
}

func (s *modifyController) syncPVC(key string) error {
	klog.InfoS("Started PVC processing", "key", key)
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return fmt.Errorf("cannot get namespace and name from key (%s): %w", key, err)
	}

	pvcObject, exists, err := s.claims.GetByKey(key)
	if err != nil {
		return fmt.Errorf("cannot get PVC for key (%s): %w", key, err)
	}

	if !exists {
		klog.InfoS("PVC is deleted or does not exist", "namespace", namespace, "name", name)
		return nil
	}

	pvc, ok := pvcObject.(*lapiCoreV1.PersistentVolumeClaim)
	if !ok {
		return fmt.Errorf("expected PVC for key (%s) but got %v", key, pvcObject)
	}

	if pvc.Spec.VolumeName == "" {
		klog.InfoS("PV bound to PVC is not created yet", "pvc", lutil.PVCKey(pvc))
		return nil
	}

	volumeObj, exists, err := s.volumes.GetByKey(pvc.Spec.VolumeName)
	if err != nil {
		return fmt.Errorf("get PV %q of pvc %q failed: %v", pvc.Spec.VolumeName, lutil.PVCKey(pvc), err)
	}

	if !exists {
		klog.Warningf("PV %q bound to PVC %s not found", pvc.Spec.VolumeName, lutil.PVCKey(pvc))
		return nil
	}

	pv, ok := volumeObj.(*lapiCoreV1.PersistentVolume)
	if !ok {
		return fmt.Errorf("expected volume but got %+v", volumeObj)
	}

	if !s.pvcNeedsModification(pv, pvc) {
		klog.InfoS("No need to modify PVC", "pvc", lutil.PVCKey(pvc))
		return nil
	}

	return s.modifyPVC(pv, pvc)
}

// Determines if the PVC needs modification.
func (s *modifyController) pvcNeedsModification(pv *lapiCoreV1.PersistentVolume, pvc *lapiCoreV1.PersistentVolumeClaim) bool {
	// Check if there's already a modification going on.
	if s.ifPVCModificationInProgress(pvc.Name) {
		klog.InfoS("modification for pvc is already undergoing", "pvc", lutil.PVCKey(pvc))
		return false
	}

	// Only Bound PVC can be modified
	if pvc.Status.Phase != lapiCoreV1.ClaimBound {
		klog.InfoS("pvc is not bound", "pvc", lutil.PVCKey(pvc))
		return false
	}

	if pvc.Spec.VolumeName == "" {
		klog.InfoS("volume name is empty", "pvc", lutil.PVCKey(pvc))
		return false
	}

	if !s.annotationsUpdated(pvc.Annotations, pv.Annotations) {
		klog.InfoS("annotations not updated", "pvc", lutil.PVCKey(pvc))
		return false
	}

	return true
}

// Check if annotations are updated.
func (s *modifyController) annotationsUpdated(ppvcAnnotations, ppvAnnotations map[string]string) bool {
	m := make(map[string]string)
	for key, value := range ppvcAnnotations {
		if s.isValidAnnotation(key) {
			m[key] = value
		}
	}

	for key, value := range m {
		if ppvAnnotations[key] != value {
			return true
		}
	}

	return false
}

func (s *modifyController) ifPVCModificationInProgress(ppvc string) bool {
	s.modificationInProgressMu.Lock()
	defer s.modificationInProgressMu.Unlock()
	_, ok := s.modificationInProgress[ppvc]
	return ok
}

func (s *modifyController) modifyPVC(pv *lapiCoreV1.PersistentVolume, pvc *lapiCoreV1.PersistentVolumeClaim) error {
	s.addPVCToInProgressList(pvc.Name)
	defer s.removePVCFromInProgressList(pvc.Name)

	params := make(map[string]string)
	for key, value := range pvc.Annotations {
		if s.isValidAnnotation(key) {
			params[s.attributeFromValidAnnotation(key)] = value
		}
	}

	reqContext := make(map[string]string)

	s.eventRecorder.Event(pvc, lapiCoreV1.EventTypeNormal, VolumeModificationStarted, fmt.Sprintf("External modifier is modifying volume %s", pv.Name))

	err := s.modifier.Modify(pv, params, reqContext)
	if err != nil {
		s.eventRecorder.Eventf(pvc, lapiCoreV1.EventTypeWarning, VolumeModificationFailed, err.Error())
		return fmt.Errorf("modification of volume %q failed by modifier %q: %w", pvc.Name, s.name, err)
	} else {
		s.eventRecorder.Eventf(pvc, lapiCoreV1.EventTypeNormal, VolumeModificationSuccessful, "External modifier has successfully modified volume %s", pv.Name)
	}

	return s.markPVCModificationComplete(pv, params)
}

func (s *modifyController) addPVCToInProgressList(pvc string) {
	s.modificationInProgressMu.Lock()
	defer s.modificationInProgressMu.Unlock()
	s.modificationInProgress[pvc] = struct{}{}
}

func (s *modifyController) removePVCFromInProgressList(pvc string) {
	s.modificationInProgressMu.Lock()
	defer s.modificationInProgressMu.Unlock()
	delete(s.modificationInProgress, pvc)
}

func (s *modifyController) attributeFromValidAnnotation(pann string) string {
	return strings.TrimPrefix(pann, fmt.Sprintf(AnnotationPrefixPattern, s.name))
}

func (s *modifyController) markPVCModificationComplete(oldPV *lapiCoreV1.PersistentVolume, params map[string]string) error {
	newPV := oldPV.DeepCopy()
	for key, value := range params {
		newPV.Annotations[fmt.Sprintf("%s/%s", s.name, key)] = value
	}

	_, err := s.patchPV(oldPV, newPV, true)
	return err
}

func (s *modifyController) patchPV(old, new *lapiCoreV1.PersistentVolume, addResourceVersionCheck bool) (*lapiCoreV1.PersistentVolume, error) {
	patchBytes, err := lutil.GetPatchData(old, new)
	if err != nil {
		return old, fmt.Errorf("can't patch status of PV %s as patch data generation failed: %v", old.Name, err)
	}

	updatedPV, err := s.kubeClient.CoreV1().PersistentVolumes().
		Patch(context.TODO(), old.Name, types.StrategicMergePatchType, patchBytes, lmetaV1.PatchOptions{})

	if err != nil {
		return old, fmt.Errorf("can't patch PV %s with %v", old.Name, err)
	}

	err = s.volumes.Update(updatedPV)
	if err != nil {
		return old, fmt.Errorf("error updating PV %s in local cache: %v", old.Name, err)
	}
	return updatedPV, nil
}

// _____________________________________________________________________________________________________________________PRIVATE METHODS

func getObjectKeys(pobj interface{}) (string, error) {
	if unknown, ok := pobj.(cache.DeletedFinalStateUnknown); ok && unknown.Obj != nil {
		pobj = unknown.Obj
	}

	objKey, err := cache.DeletionHandlingMetaNamespaceKeyFunc(pobj)
	if err != nil {
		klog.Errorf("Failed to get key from object: %v", err)
		return "", err
	}
	return objKey, nil
}
