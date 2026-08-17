package service

import (
	"context"
	"encoding/json"
	"strings"

	"test-auto-pro-v2/internal/model"
)

// CopyCycles 把当前计划内已保存的循环复制到目标路径，只接受完整结构签名一致的路径。
// 来源和目标动作仍由各自节点独立保存；复制操作只写循环命名空间，不调用目标平台。
func (s *PathConfigService) CopyCycles(ctx context.Context, planID, targetPathID, sourcePathID uint64, idempotencyKey string) (model.PathConfigSaveResult, error) {
	if planID == 0 || targetPathID == 0 || sourcePathID == 0 || targetPathID == sourcePathID || !validUUID(strings.TrimSpace(idempotencyKey)) {
		return model.PathConfigSaveResult{}, &PathConfigError{Kind: PathConfigErrorInvalidArgument, Message: "循环复制参数不正确"}
	}
	if err := s.validateConfigMutablePlan(ctx, planID); err != nil {
		return model.PathConfigSaveResult{}, err
	}
	targetPath, err := s.ownedPath(ctx, planID, targetPathID)
	if err != nil {
		return model.PathConfigSaveResult{}, err
	}
	sourcePath, err := s.ownedPath(ctx, planID, sourcePathID)
	if err != nil {
		return model.PathConfigSaveResult{}, err
	}
	if pathConfigStructuralSignature(targetPath) != pathConfigStructuralSignature(sourcePath) {
		return model.PathConfigSaveResult{}, &PathConfigError{Kind: PathConfigErrorInvalid, Message: "只有流程结构完全一致的路径才能复制循环"}
	}
	if existing, found, findErr := s.configRepository.FindByPathAndKey(ctx, targetPathID, idempotencyKey); findErr != nil {
		return model.PathConfigSaveResult{}, mapPathConfigRepositoryError(findErr)
	} else if found {
		return pathConfigSaveResult(targetPath, existing), nil
	}
	sourceStored, sourceFound, err := s.configRepository.FindByPath(ctx, sourcePathID)
	if err != nil {
		return model.PathConfigSaveResult{}, mapPathConfigRepositoryError(err)
	}
	if !sourceFound {
		return model.PathConfigSaveResult{}, &PathConfigError{Kind: PathConfigErrorInvalid, Message: "来源路径尚未保存循环"}
	}
	cycleInputs := decodePathConfigActionCycleInputs(sourceStored.ActionValues)
	if cycleInputs == nil || len(cycleInputs) == 0 {
		return model.PathConfigSaveResult{}, &PathConfigError{Kind: PathConfigErrorInvalid, Message: "来源路径尚未保存有效循环"}
	}
	snapshot, err := s.readVerifiedSnapshot(ctx, planID)
	if err != nil {
		return model.PathConfigSaveResult{}, err
	}
	owned, err := s.analyzeOwnedPath(ctx, planID, snapshot, targetPath)
	if err != nil {
		return model.PathConfigSaveResult{}, err
	}
	targetStored, targetFound, err := s.configRepository.FindByPath(ctx, targetPathID)
	if err != nil {
		return model.PathConfigSaveResult{}, mapPathConfigRepositoryError(err)
	}
	if !targetFound {
		targetStored = model.StoredPathConfig{PathID: targetPathID, FieldValues: map[string]map[string]string{}, ActionValues: map[string]string{}}
	}
	configuration, _, err := s.configAnalyzer.Analyze(owned.graph, snapshot.Tree, snapshot.FormFields, targetPath, owned.pathAnalysis, snapshot.InstanceValues, targetStored.FieldValues, targetStored.ActionValues, targetFound)
	if err != nil {
		return model.PathConfigSaveResult{}, &PathConfigError{Kind: PathConfigErrorInvalid, Message: "目标路径无法核对循环"}
	}
	if _, cycleErr := validatePathConfigActionCycles(configuration, cycleInputs); cycleErr != nil {
		return model.PathConfigSaveResult{}, cycleErr
	}
	encoded, err := json.Marshal(cycleInputs)
	if err != nil {
		return model.PathConfigSaveResult{}, &PathConfigError{Kind: PathConfigErrorStorage, Message: "循环复制暂时无法保存，请重试"}
	}
	values := copyPathConfigActionValues(targetStored.ActionValues)
	values[pathConfigActionCyclesStorageKey] = string(encoded)
	targetStored.PathID, targetStored.ActionValues = targetPathID, values
	targetStored.Revision, targetStored.NodeRevision, targetStored.ConfigVersion = targetStored.Revision+1, targetStored.NodeRevision+1, currentPathConfigVersion
	targetStored.IdempotencyKey = idempotencyKey
	targetStored.Status = s.deriveStoredStatus(ctx, planID, targetPath, snapshot, targetStored)
	saved, err := s.configRepository.Save(ctx, targetStored, targetStored.Revision-1, s.now().UTC())
	if err != nil {
		return model.PathConfigSaveResult{}, mapPathConfigRepositoryError(err)
	}
	return pathConfigSaveResult(targetPath, saved), nil
}
