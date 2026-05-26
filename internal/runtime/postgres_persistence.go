// postgres_persistence.go implements core runtime persistence on PostgreSQL.
// postgres_persistence.go 实现 PostgreSQL 版核心 runtime 持久化。
package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// PostgresRuntimeStore persists core task-run records in PostgreSQL.
// PostgresRuntimeStore 会把核心 task-run 记录持久化到 PostgreSQL。
type PostgresRuntimeStore struct {
	db *gorm.DB
}

// NewPostgresRuntimeStore creates one PostgreSQL-backed runtime persistence store.
// NewPostgresRuntimeStore 创建一个 PostgreSQL 版 runtime 持久化 store。
func NewPostgresRuntimeStore(db *gorm.DB) *PostgresRuntimeStore {
	return &PostgresRuntimeStore{db: db}
}

// AutoMigrate creates or updates the runtime persistence tables.
// AutoMigrate 创建或更新 runtime 持久化表结构。
func (s *PostgresRuntimeStore) AutoMigrate(ctx context.Context) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("postgres runtime store is not configured")
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return tx.AutoMigrate(
			&postgresRuntimeContractModel{},
			&postgresTaskTypeRegistrationModel{},
			&postgresHookBindingModel{},
			&postgresSystemTruthSourceModel{},
			&postgresSystemTruthDraftModel{},
			&postgresSystemTruthCompileResultModel{},
			&postgresSystemTruthActiveVersionModel{},
			&postgresTaskRunModel{},
			&postgresTaskStepModel{},
			&postgresRuntimeTraceModel{},
			&postgresUsageModel{},
			&postgresLifecycleEventModel{},
			&postgresProjectionCandidateModel{},
			&postgresRuntimeGraphCheckpointModel{},
		)
	})
}

// WithTransaction runs runtime persistence operations through one database transaction.
// WithTransaction 会在一个数据库事务中执行 runtime 持久化操作。
func (s *PostgresRuntimeStore) WithTransaction(ctx context.Context, fn func(RuntimePersistenceStore) error) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("postgres runtime store is not configured")
	}
	if fn == nil {
		return fmt.Errorf("runtime persistence transaction function is required")
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(NewPostgresRuntimeStore(tx))
	})
}

// CreateTaskTypeRegistration inserts one registered task type.
// CreateTaskTypeRegistration 插入一个 registered task type。
func (s *PostgresRuntimeStore) CreateTaskTypeRegistration(ctx context.Context, input TaskTypeRegistration) (TaskTypeRegistration, error) {
	if s == nil || s.db == nil {
		return TaskTypeRegistration{}, fmt.Errorf("postgres runtime store is not configured")
	}
	if err := validateTaskTypeRegistration(input); err != nil {
		return TaskTypeRegistration{}, err
	}
	row, err := taskTypeRegistrationToRow(input)
	if err != nil {
		return TaskTypeRegistration{}, err
	}
	if err := s.db.WithContext(ctx).Create(&row).Error; err != nil {
		return TaskTypeRegistration{}, err
	}
	return taskTypeRegistrationFromRow(row)
}

// PutTaskTypeRegistration creates or replaces one registered task type by primary key.
// PutTaskTypeRegistration 按主键创建或替换一条 registered task type 记录。
func (s *PostgresRuntimeStore) PutTaskTypeRegistration(ctx context.Context, input TaskTypeRegistration) (TaskTypeRegistration, error) {
	if s == nil || s.db == nil {
		return TaskTypeRegistration{}, fmt.Errorf("postgres runtime store is not configured")
	}
	if err := validateTaskTypeRegistration(input); err != nil {
		return TaskTypeRegistration{}, err
	}
	row, err := taskTypeRegistrationToRow(input)
	if err != nil {
		return TaskTypeRegistration{}, err
	}
	if err := s.db.WithContext(ctx).Save(&row).Error; err != nil {
		return TaskTypeRegistration{}, err
	}
	return taskTypeRegistrationFromRow(row)
}

// GetTaskTypeRegistration loads one task type registration by ID.
// GetTaskTypeRegistration 按 ID 读取一个 task type registration。
func (s *PostgresRuntimeStore) GetTaskTypeRegistration(ctx context.Context, id string) (TaskTypeRegistration, bool, error) {
	if s == nil || s.db == nil {
		return TaskTypeRegistration{}, false, fmt.Errorf("postgres runtime store is not configured")
	}
	var row postgresTaskTypeRegistrationModel
	err := s.db.WithContext(ctx).First(&row, "id = ?", strings.TrimSpace(id)).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return TaskTypeRegistration{}, false, nil
		}
		return TaskTypeRegistration{}, false, err
	}
	output, err := taskTypeRegistrationFromRow(row)
	return output, err == nil, err
}

// GetTaskTypeRegistrationByKey loads one task type registration by stable type key.
// GetTaskTypeRegistrationByKey 按稳定 type key 读取一个 task type registration。
func (s *PostgresRuntimeStore) GetTaskTypeRegistrationByKey(ctx context.Context, typeKey string) (TaskTypeRegistration, bool, error) {
	if s == nil || s.db == nil {
		return TaskTypeRegistration{}, false, fmt.Errorf("postgres runtime store is not configured")
	}
	var row postgresTaskTypeRegistrationModel
	err := s.db.WithContext(ctx).First(&row, "type_key = ?", strings.TrimSpace(typeKey)).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return TaskTypeRegistration{}, false, nil
		}
		return TaskTypeRegistration{}, false, err
	}
	output, err := taskTypeRegistrationFromRow(row)
	return output, err == nil, err
}

// ListTaskTypeRegistrations lists registered task types.
// ListTaskTypeRegistrations 列出 registered task types。
func (s *PostgresRuntimeStore) ListTaskTypeRegistrations(ctx context.Context, filter TaskTypeRegistrationListFilter) ([]TaskTypeRegistration, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("postgres runtime store is not configured")
	}
	query := s.db.WithContext(ctx).Model(&postgresTaskTypeRegistrationModel{})
	if strings.TrimSpace(filter.Status) != "" {
		query = query.Where("status = ?", strings.TrimSpace(filter.Status))
	}
	if filter.Limit > 0 {
		query = query.Limit(filter.Limit)
	}
	var rows []postgresTaskTypeRegistrationModel
	if err := query.Order("type_key asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	return taskTypeRegistrationsFromRows(rows)
}

// CreateHookBinding inserts one internal allowlisted hook binding.
// CreateHookBinding 插入一个内部 allowlisted hook binding。
func (s *PostgresRuntimeStore) CreateHookBinding(ctx context.Context, input HookBinding) (HookBinding, error) {
	if s == nil || s.db == nil {
		return HookBinding{}, fmt.Errorf("postgres runtime store is not configured")
	}
	if err := validateHookBinding(input); err != nil {
		return HookBinding{}, err
	}
	row, err := hookBindingToRow(input)
	if err != nil {
		return HookBinding{}, err
	}
	if err := s.db.WithContext(ctx).Create(&row).Error; err != nil {
		return HookBinding{}, err
	}
	return hookBindingFromRow(row)
}

// PutHookBinding creates or replaces one internal allowlisted hook binding by primary key.
// PutHookBinding 按主键创建或替换一条内部 allowlisted hook binding。
func (s *PostgresRuntimeStore) PutHookBinding(ctx context.Context, input HookBinding) (HookBinding, error) {
	if s == nil || s.db == nil {
		return HookBinding{}, fmt.Errorf("postgres runtime store is not configured")
	}
	if err := validateHookBinding(input); err != nil {
		return HookBinding{}, err
	}
	row, err := hookBindingToRow(input)
	if err != nil {
		return HookBinding{}, err
	}
	if err := s.db.WithContext(ctx).Save(&row).Error; err != nil {
		return HookBinding{}, err
	}
	return hookBindingFromRow(row)
}

// GetHookBinding loads one hook binding by ID.
// GetHookBinding 按 ID 读取一个 hook binding。
func (s *PostgresRuntimeStore) GetHookBinding(ctx context.Context, id string) (HookBinding, bool, error) {
	if s == nil || s.db == nil {
		return HookBinding{}, false, fmt.Errorf("postgres runtime store is not configured")
	}
	var row postgresHookBindingModel
	err := s.db.WithContext(ctx).First(&row, "id = ?", strings.TrimSpace(id)).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return HookBinding{}, false, nil
		}
		return HookBinding{}, false, err
	}
	output, err := hookBindingFromRow(row)
	return output, err == nil, err
}

// ListHookBindings lists hook bindings in deterministic execution order.
// ListHookBindings 按确定性执行顺序列出 hook bindings。
func (s *PostgresRuntimeStore) ListHookBindings(ctx context.Context, filter HookBindingListFilter) ([]HookBinding, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("postgres runtime store is not configured")
	}
	query := s.db.WithContext(ctx).Model(&postgresHookBindingModel{})
	if strings.TrimSpace(filter.ContractID) != "" {
		query = query.Where("contract_id = ?", strings.TrimSpace(filter.ContractID))
	}
	if strings.TrimSpace(filter.HookPoint) != "" {
		query = query.Where("hook_point = ?", strings.TrimSpace(filter.HookPoint))
	}
	if filter.Enabled != nil {
		query = query.Where("enabled = ?", *filter.Enabled)
	}
	if filter.Limit > 0 {
		query = query.Limit(filter.Limit)
	}
	var rows []postgresHookBindingModel
	if err := query.Order("order_index asc, created_at asc, id asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	return hookBindingsFromRows(rows)
}

// CreateSystemTruthSource inserts one system truth source lifecycle record.
// CreateSystemTruthSource 插入一个 system truth source 生命周期记录。
func (s *PostgresRuntimeStore) CreateSystemTruthSource(ctx context.Context, input SystemTruthSource) (SystemTruthSource, error) {
	if s == nil || s.db == nil {
		return SystemTruthSource{}, fmt.Errorf("postgres runtime store is not configured")
	}
	if err := validateSystemTruthSource(input); err != nil {
		return SystemTruthSource{}, err
	}
	row, err := systemTruthSourceToRow(input)
	if err != nil {
		return SystemTruthSource{}, err
	}
	if err := s.db.WithContext(ctx).Create(&row).Error; err != nil {
		return SystemTruthSource{}, err
	}
	return systemTruthSourceFromRow(row)
}

// CreateSystemTruthDraft inserts one editable system truth draft record.
// CreateSystemTruthDraft 插入一条可编辑 system truth draft 记录。
func (s *PostgresRuntimeStore) CreateSystemTruthDraft(ctx context.Context, input SystemTruthDraft) (SystemTruthDraft, error) {
	if s == nil || s.db == nil {
		return SystemTruthDraft{}, fmt.Errorf("postgres runtime store is not configured")
	}
	if err := validateSystemTruthDraft(input); err != nil {
		return SystemTruthDraft{}, err
	}
	row, err := systemTruthDraftToRow(input)
	if err != nil {
		return SystemTruthDraft{}, err
	}
	if err := s.db.WithContext(ctx).Create(&row).Error; err != nil {
		return SystemTruthDraft{}, err
	}
	return systemTruthDraftFromRow(row)
}

// CreateSystemTruthCompileResult inserts one compile result and diagnostics record.
// CreateSystemTruthCompileResult 插入一条 compile result 与 diagnostics 记录。
func (s *PostgresRuntimeStore) CreateSystemTruthCompileResult(ctx context.Context, input SystemTruthCompileResult) (SystemTruthCompileResult, error) {
	if s == nil || s.db == nil {
		return SystemTruthCompileResult{}, fmt.Errorf("postgres runtime store is not configured")
	}
	if err := validateSystemTruthCompileResult(input); err != nil {
		return SystemTruthCompileResult{}, err
	}
	row, err := systemTruthCompileResultToRow(input)
	if err != nil {
		return SystemTruthCompileResult{}, err
	}
	if err := s.db.WithContext(ctx).Create(&row).Error; err != nil {
		return SystemTruthCompileResult{}, err
	}
	return systemTruthCompileResultFromRow(row)
}

// ActivateSystemTruthVersion inserts one audited active system truth pointer.
// ActivateSystemTruthVersion 插入一个可审计的 active system truth pointer。
func (s *PostgresRuntimeStore) ActivateSystemTruthVersion(ctx context.Context, input SystemTruthActiveVersion) (SystemTruthActiveVersion, error) {
	if s == nil || s.db == nil {
		return SystemTruthActiveVersion{}, fmt.Errorf("postgres runtime store is not configured")
	}
	if err := validateSystemTruthActiveVersion(input); err != nil {
		return SystemTruthActiveVersion{}, err
	}
	var compileRow postgresSystemTruthCompileResultModel
	err := s.db.WithContext(ctx).First(&compileRow, "id = ?", strings.TrimSpace(input.CompileResultID)).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return SystemTruthActiveVersion{}, invalidRuntimePersistenceInput("system truth compile result %q not found", input.CompileResultID)
		}
		return SystemTruthActiveVersion{}, err
	}
	if compileRow.Status != SystemTruthCompileStatusSucceeded {
		return SystemTruthActiveVersion{}, invalidRuntimePersistenceInput("system truth compile result %q is not successful", input.CompileResultID)
	}
	row, err := systemTruthActiveVersionToRow(input)
	if err != nil {
		return SystemTruthActiveVersion{}, err
	}
	if err := s.db.WithContext(ctx).Create(&row).Error; err != nil {
		return SystemTruthActiveVersion{}, err
	}
	return systemTruthActiveVersionFromRow(row)
}

// GetActiveSystemTruthVersion loads the latest active pointer for one asset.
// GetActiveSystemTruthVersion 读取某个 asset 的最新 active pointer。
func (s *PostgresRuntimeStore) GetActiveSystemTruthVersion(ctx context.Context, assetID string) (SystemTruthActiveVersion, bool, error) {
	if s == nil || s.db == nil {
		return SystemTruthActiveVersion{}, false, fmt.Errorf("postgres runtime store is not configured")
	}
	var row postgresSystemTruthActiveVersionModel
	err := s.db.WithContext(ctx).
		Where("asset_id = ?", strings.TrimSpace(assetID)).
		Order("activated_at desc, id desc").
		First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return SystemTruthActiveVersion{}, false, nil
		}
		return SystemTruthActiveVersion{}, false, err
	}
	output, err := systemTruthActiveVersionFromRow(row)
	return output, err == nil, err
}

// ListSystemTruthActiveVersions lists active pointer changes for one asset, or all assets when assetID is empty.
// ListSystemTruthActiveVersions 列出某个 asset 的 active pointer 变更；assetID 为空时列出全部。
func (s *PostgresRuntimeStore) ListSystemTruthActiveVersions(ctx context.Context, assetID string) ([]SystemTruthActiveVersion, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("postgres runtime store is not configured")
	}
	var rows []postgresSystemTruthActiveVersionModel
	query := s.db.WithContext(ctx).Model(&postgresSystemTruthActiveVersionModel{})
	if strings.TrimSpace(assetID) != "" {
		query = query.Where("asset_id = ?", strings.TrimSpace(assetID))
	}
	if err := query.Order("activated_at desc, id desc").Find(&rows).Error; err != nil {
		return nil, err
	}
	return systemTruthActiveVersionsFromRows(rows)
}

// CreateRuntimeContract inserts one runtime contract envelope.
// CreateRuntimeContract 插入一个 runtime contract 契约包络。
func (s *PostgresRuntimeStore) CreateRuntimeContract(ctx context.Context, input RuntimeContract) (RuntimeContract, error) {
	if s == nil || s.db == nil {
		return RuntimeContract{}, fmt.Errorf("postgres runtime store is not configured")
	}
	if err := validateRuntimeContract(input); err != nil {
		return RuntimeContract{}, err
	}
	row, err := runtimeContractToRow(input)
	if err != nil {
		return RuntimeContract{}, err
	}
	if err := s.db.WithContext(ctx).Create(&row).Error; err != nil {
		return RuntimeContract{}, err
	}
	return runtimeContractFromRow(row)
}

// PutRuntimeContract creates or replaces one runtime contract envelope by primary key.
// PutRuntimeContract 按主键创建或替换一个 runtime contract 契约包络。
func (s *PostgresRuntimeStore) PutRuntimeContract(ctx context.Context, input RuntimeContract) (RuntimeContract, error) {
	if s == nil || s.db == nil {
		return RuntimeContract{}, fmt.Errorf("postgres runtime store is not configured")
	}
	if err := validateRuntimeContract(input); err != nil {
		return RuntimeContract{}, err
	}
	row, err := runtimeContractToRow(input)
	if err != nil {
		return RuntimeContract{}, err
	}
	if err := s.db.WithContext(ctx).Save(&row).Error; err != nil {
		return RuntimeContract{}, err
	}
	return runtimeContractFromRow(row)
}

// GetRuntimeContract loads one runtime contract by ID.
// GetRuntimeContract 按 ID 读取一个 runtime contract。
func (s *PostgresRuntimeStore) GetRuntimeContract(ctx context.Context, id string) (RuntimeContract, bool, error) {
	if s == nil || s.db == nil {
		return RuntimeContract{}, false, fmt.Errorf("postgres runtime store is not configured")
	}
	var row postgresRuntimeContractModel
	err := s.db.WithContext(ctx).First(&row, "id = ?", strings.TrimSpace(id)).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return RuntimeContract{}, false, nil
		}
		return RuntimeContract{}, false, err
	}
	output, err := runtimeContractFromRow(row)
	return output, err == nil, err
}

// ListRuntimeContracts lists runtime contracts by stable typed fields.
// ListRuntimeContracts 按稳定 typed 字段列出 runtime contracts。
func (s *PostgresRuntimeStore) ListRuntimeContracts(ctx context.Context, filter RuntimeContractListFilter) ([]RuntimeContract, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("postgres runtime store is not configured")
	}
	query := s.db.WithContext(ctx).Model(&postgresRuntimeContractModel{})
	if strings.TrimSpace(filter.TaskType) != "" {
		query = query.Where("task_type = ?", strings.TrimSpace(filter.TaskType))
	}
	if strings.TrimSpace(filter.Status) != "" {
		query = query.Where("status = ?", strings.TrimSpace(filter.Status))
	}
	if filter.Limit > 0 {
		query = query.Limit(filter.Limit)
	}
	var rows []postgresRuntimeContractModel
	if err := query.Order("updated_at desc, created_at desc, id asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	return runtimeContractsFromRows(rows)
}

// CreateTaskRun inserts one task-run aggregate root.
// CreateTaskRun 插入一个 task-run 聚合根。
func (s *PostgresRuntimeStore) CreateTaskRun(ctx context.Context, input TaskRun) (TaskRun, error) {
	if s == nil || s.db == nil {
		return TaskRun{}, fmt.Errorf("postgres runtime store is not configured")
	}
	if err := validateTaskRun(input); err != nil {
		return TaskRun{}, err
	}
	row, err := taskRunToRow(input)
	if err != nil {
		return TaskRun{}, err
	}
	if err := s.db.WithContext(ctx).Create(&row).Error; err != nil {
		return TaskRun{}, err
	}
	return taskRunFromRow(row)
}

// GetTaskRun loads one task run by ID.
// GetTaskRun 按 ID 读取一条 task run。
func (s *PostgresRuntimeStore) GetTaskRun(ctx context.Context, id string) (TaskRun, bool, error) {
	if s == nil || s.db == nil {
		return TaskRun{}, false, fmt.Errorf("postgres runtime store is not configured")
	}
	var row postgresTaskRunModel
	err := s.db.WithContext(ctx).First(&row, "id = ?", strings.TrimSpace(id)).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return TaskRun{}, false, nil
		}
		return TaskRun{}, false, err
	}
	output, err := taskRunFromRow(row)
	return output, err == nil, err
}

// ListTaskRuns lists task runs by stable typed fields.
// ListTaskRuns 按稳定 typed 字段列出 task run。
func (s *PostgresRuntimeStore) ListTaskRuns(ctx context.Context, filter TaskRunListFilter) ([]TaskRun, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("postgres runtime store is not configured")
	}
	query := s.db.WithContext(ctx).Model(&postgresTaskRunModel{})
	if strings.TrimSpace(filter.WorkspaceID) != "" {
		query = query.Where("workspace_id = ?", strings.TrimSpace(filter.WorkspaceID))
	}
	if strings.TrimSpace(filter.Status) != "" {
		query = query.Where("status = ?", strings.TrimSpace(filter.Status))
	}
	if filter.Limit > 0 {
		query = query.Limit(filter.Limit)
	}
	var rows []postgresTaskRunModel
	if err := query.Order("created_at desc, id asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	return taskRunsFromRows(rows)
}

// CreateTaskStep inserts one step under a run.
// CreateTaskStep 会在 run 下插入一条 step。
func (s *PostgresRuntimeStore) CreateTaskStep(ctx context.Context, input TaskStep) (TaskStep, error) {
	if s == nil || s.db == nil {
		return TaskStep{}, fmt.Errorf("postgres runtime store is not configured")
	}
	if err := validateTaskStep(input); err != nil {
		return TaskStep{}, err
	}
	row, err := taskStepToRow(input)
	if err != nil {
		return TaskStep{}, err
	}
	if err := s.db.WithContext(ctx).Create(&row).Error; err != nil {
		return TaskStep{}, err
	}
	return taskStepFromRow(row)
}

// GetTaskStep loads one task step by ID.
// GetTaskStep 按 ID 读取一条 task step。
func (s *PostgresRuntimeStore) GetTaskStep(ctx context.Context, id string) (TaskStep, bool, error) {
	if s == nil || s.db == nil {
		return TaskStep{}, false, fmt.Errorf("postgres runtime store is not configured")
	}
	var row postgresTaskStepModel
	err := s.db.WithContext(ctx).First(&row, "id = ?", strings.TrimSpace(id)).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return TaskStep{}, false, nil
		}
		return TaskStep{}, false, err
	}
	output, err := taskStepFromRow(row)
	return output, err == nil, err
}

// ListTaskSteps lists steps for one run in sequence order.
// ListTaskSteps 按顺序列出一个 run 下的 steps。
func (s *PostgresRuntimeStore) ListTaskSteps(ctx context.Context, runID string) ([]TaskStep, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("postgres runtime store is not configured")
	}
	var rows []postgresTaskStepModel
	if err := s.db.WithContext(ctx).Where("run_id = ?", strings.TrimSpace(runID)).Order("sequence asc, created_at asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	return taskStepsFromRows(rows)
}

// CreateLifecycleEvent inserts one run or step lifecycle transition event.
// CreateLifecycleEvent 插入一条 run 或 step 生命周期转换事件。
func (s *PostgresRuntimeStore) CreateLifecycleEvent(ctx context.Context, input TaskRunLifecycleEvent) (TaskRunLifecycleEvent, error) {
	if s == nil || s.db == nil {
		return TaskRunLifecycleEvent{}, fmt.Errorf("postgres runtime store is not configured")
	}
	if err := validateLifecycleEvent(input); err != nil {
		return TaskRunLifecycleEvent{}, err
	}
	row, err := lifecycleEventToRow(input)
	if err != nil {
		return TaskRunLifecycleEvent{}, err
	}
	if err := s.db.WithContext(ctx).Create(&row).Error; err != nil {
		return TaskRunLifecycleEvent{}, err
	}
	return lifecycleEventFromRow(row)
}

// GetLifecycleEvent loads one lifecycle event by ID.
// GetLifecycleEvent 按 ID 读取一条生命周期事件。
func (s *PostgresRuntimeStore) GetLifecycleEvent(ctx context.Context, id string) (TaskRunLifecycleEvent, bool, error) {
	if s == nil || s.db == nil {
		return TaskRunLifecycleEvent{}, false, fmt.Errorf("postgres runtime store is not configured")
	}
	var row postgresLifecycleEventModel
	err := s.db.WithContext(ctx).First(&row, "id = ?", strings.TrimSpace(id)).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return TaskRunLifecycleEvent{}, false, nil
		}
		return TaskRunLifecycleEvent{}, false, err
	}
	output, err := lifecycleEventFromRow(row)
	return output, err == nil, err
}

// ListLifecycleEventsByRun lists all lifecycle events for a run.
// ListLifecycleEventsByRun 列出一个 run 的全部生命周期事件。
func (s *PostgresRuntimeStore) ListLifecycleEventsByRun(ctx context.Context, runID string) ([]TaskRunLifecycleEvent, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("postgres runtime store is not configured")
	}
	var rows []postgresLifecycleEventModel
	if err := s.db.WithContext(ctx).Where("run_id = ?", strings.TrimSpace(runID)).Order("occurred_at asc, id asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	return lifecycleEventsFromRows(rows)
}

// ListLifecycleEventsBySubject lists lifecycle events attached to one run or step subject.
// ListLifecycleEventsBySubject 列出一个 run 或 step 主体的生命周期事件。
func (s *PostgresRuntimeStore) ListLifecycleEventsBySubject(ctx context.Context, subjectType string, subjectID string) ([]TaskRunLifecycleEvent, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("postgres runtime store is not configured")
	}
	var rows []postgresLifecycleEventModel
	if err := s.db.WithContext(ctx).
		Where("subject_type = ? AND subject_id = ?", strings.TrimSpace(subjectType), strings.TrimSpace(subjectID)).
		Order("occurred_at asc, id asc").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	return lifecycleEventsFromRows(rows)
}

// CreateRuntimeTrace inserts one safe trace summary.
// CreateRuntimeTrace 插入一条安全 trace 摘要。
func (s *PostgresRuntimeStore) CreateRuntimeTrace(ctx context.Context, input RuntimeTrace) (RuntimeTrace, error) {
	if s == nil || s.db == nil {
		return RuntimeTrace{}, fmt.Errorf("postgres runtime store is not configured")
	}
	if err := validateRuntimeTrace(input); err != nil {
		return RuntimeTrace{}, err
	}
	row, err := runtimeTraceToRow(input)
	if err != nil {
		return RuntimeTrace{}, err
	}
	if err := s.db.WithContext(ctx).Create(&row).Error; err != nil {
		return RuntimeTrace{}, err
	}
	return runtimeTraceFromRow(row)
}

// GetRuntimeTrace loads one runtime trace by ID.
// GetRuntimeTrace 按 ID 读取一条 runtime trace。
func (s *PostgresRuntimeStore) GetRuntimeTrace(ctx context.Context, id string) (RuntimeTrace, bool, error) {
	if s == nil || s.db == nil {
		return RuntimeTrace{}, false, fmt.Errorf("postgres runtime store is not configured")
	}
	var row postgresRuntimeTraceModel
	err := s.db.WithContext(ctx).First(&row, "id = ?", strings.TrimSpace(id)).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return RuntimeTrace{}, false, nil
		}
		return RuntimeTrace{}, false, err
	}
	output, err := runtimeTraceFromRow(row)
	return output, err == nil, err
}

// ListRuntimeTraces lists traces for one run and optional step.
// ListRuntimeTraces 列出一个 run 和可选 step 下的 trace。
func (s *PostgresRuntimeStore) ListRuntimeTraces(ctx context.Context, filter RuntimeTraceListFilter) ([]RuntimeTrace, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("postgres runtime store is not configured")
	}
	query := s.db.WithContext(ctx).Where("run_id = ?", strings.TrimSpace(filter.RunID))
	if strings.TrimSpace(filter.StepID) != "" {
		query = query.Where("step_id = ?", strings.TrimSpace(filter.StepID))
	}
	if filter.Limit > 0 {
		query = query.Limit(filter.Limit)
	}
	var rows []postgresRuntimeTraceModel
	if err := query.Order("created_at asc, id asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	return runtimeTracesFromRows(rows)
}

// CreateUsage inserts one generic resource-usage record.
// CreateUsage 插入一条通用资源用量记录。
func (s *PostgresRuntimeStore) CreateUsage(ctx context.Context, input Usage) (Usage, error) {
	if s == nil || s.db == nil {
		return Usage{}, fmt.Errorf("postgres runtime store is not configured")
	}
	if err := validateUsage(input); err != nil {
		return Usage{}, err
	}
	row, err := usageToRow(input)
	if err != nil {
		return Usage{}, err
	}
	if err := s.db.WithContext(ctx).Create(&row).Error; err != nil {
		return Usage{}, err
	}
	return usageFromRow(row)
}

// GetUsage loads one usage record by ID.
// GetUsage 按 ID 读取一条 usage 记录。
func (s *PostgresRuntimeStore) GetUsage(ctx context.Context, id string) (Usage, bool, error) {
	if s == nil || s.db == nil {
		return Usage{}, false, fmt.Errorf("postgres runtime store is not configured")
	}
	var row postgresUsageModel
	err := s.db.WithContext(ctx).First(&row, "id = ?", strings.TrimSpace(id)).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return Usage{}, false, nil
		}
		return Usage{}, false, err
	}
	output, err := usageFromRow(row)
	return output, err == nil, err
}

// ListUsage lists generic usage records for one run and optional step.
// ListUsage 列出一个 run 和可选 step 下的通用 usage 记录。
func (s *PostgresRuntimeStore) ListUsage(ctx context.Context, filter UsageListFilter) ([]Usage, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("postgres runtime store is not configured")
	}
	query := s.db.WithContext(ctx).Where("run_id = ?", strings.TrimSpace(filter.RunID))
	if strings.TrimSpace(filter.StepID) != "" {
		query = query.Where("step_id = ?", strings.TrimSpace(filter.StepID))
	}
	if filter.Limit > 0 {
		query = query.Limit(filter.Limit)
	}
	var rows []postgresUsageModel
	if err := query.Order("created_at asc, id asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	return usagesFromRows(rows)
}

// CreateProjectionCandidate inserts one minimal candidate-output projection.
// CreateProjectionCandidate 插入一条最小候选输出投影。
func (s *PostgresRuntimeStore) CreateProjectionCandidate(ctx context.Context, input ProjectionCandidate) (ProjectionCandidate, error) {
	if s == nil || s.db == nil {
		return ProjectionCandidate{}, fmt.Errorf("postgres runtime store is not configured")
	}
	input = normalizeProjectionCandidate(input)
	if err := validateProjectionCandidate(input); err != nil {
		return ProjectionCandidate{}, err
	}
	row, err := projectionCandidateToRow(input)
	if err != nil {
		return ProjectionCandidate{}, err
	}
	if err := s.db.WithContext(ctx).Create(&row).Error; err != nil {
		return ProjectionCandidate{}, err
	}
	return projectionCandidateFromRow(row)
}

// GetProjectionCandidate loads one candidate-output projection by ID.
// GetProjectionCandidate 按 ID 读取一条候选输出投影。
func (s *PostgresRuntimeStore) GetProjectionCandidate(ctx context.Context, id string) (ProjectionCandidate, bool, error) {
	if s == nil || s.db == nil {
		return ProjectionCandidate{}, false, fmt.Errorf("postgres runtime store is not configured")
	}
	var row postgresProjectionCandidateModel
	err := s.db.WithContext(ctx).First(&row, "id = ?", strings.TrimSpace(id)).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ProjectionCandidate{}, false, nil
		}
		return ProjectionCandidate{}, false, err
	}
	output, err := projectionCandidateFromRow(row)
	return output, err == nil, err
}

// ListProjectionCandidates lists candidate outputs for one run and optional step.
// ListProjectionCandidates 列出一个 run 和可选 step 下的候选输出。
func (s *PostgresRuntimeStore) ListProjectionCandidates(ctx context.Context, filter ProjectionCandidateListFilter) ([]ProjectionCandidate, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("postgres runtime store is not configured")
	}
	query := s.db.WithContext(ctx).Where("run_id = ?", strings.TrimSpace(filter.RunID))
	if strings.TrimSpace(filter.StepID) != "" {
		query = query.Where("step_id = ?", strings.TrimSpace(filter.StepID))
	}
	if filter.Limit > 0 {
		query = query.Limit(filter.Limit)
	}
	var rows []postgresProjectionCandidateModel
	if err := query.Order("created_at asc, id asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	return projectionCandidatesFromRows(rows)
}

// Get loads one private Eino checkpoint payload by ID.
// Get 按 ID 读取一份私有 Eino checkpoint payload。
func (s *PostgresRuntimeStore) Get(ctx context.Context, checkpointID string) ([]byte, bool, error) {
	if s == nil || s.db == nil {
		return nil, false, fmt.Errorf("postgres runtime store is not configured")
	}
	key := strings.TrimSpace(checkpointID)
	if key == "" {
		return nil, false, fmt.Errorf("%w: checkpoint id is required", ErrRuntimeCheckpointRejected)
	}
	var row postgresRuntimeGraphCheckpointModel
	err := s.db.WithContext(ctx).First(&row, "checkpoint_id = ?", key).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return append([]byte(nil), row.Payload...), true, nil
}

// GetCheckpointSnapshot loads safe checkpoint metadata without reading private payload bytes.
// GetCheckpointSnapshot 读取不含私有 payload 字节的 checkpoint 安全元数据。
func (s *PostgresRuntimeStore) GetCheckpointSnapshot(ctx context.Context, checkpointID string) (RuntimeGraphCheckpointSnapshot, bool, error) {
	if s == nil || s.db == nil {
		return RuntimeGraphCheckpointSnapshot{}, false, fmt.Errorf("postgres runtime store is not configured")
	}
	key := strings.TrimSpace(checkpointID)
	if key == "" {
		return RuntimeGraphCheckpointSnapshot{}, false, fmt.Errorf("%w: checkpoint id is required", ErrRuntimeCheckpointRejected)
	}
	var row postgresRuntimeGraphCheckpointModel
	err := s.db.WithContext(ctx).
		Select("checkpoint_id", "payload_size", "payload_sha256", "created_at", "updated_at").
		First(&row, "checkpoint_id = ?", key).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return RuntimeGraphCheckpointSnapshot{}, false, nil
		}
		return RuntimeGraphCheckpointSnapshot{}, false, err
	}
	return RuntimeGraphCheckpointSnapshot{
		CheckpointID:  row.CheckpointID,
		PayloadSize:   row.PayloadSize,
		PayloadSHA256: row.PayloadSHA256,
		CreatedAt:     row.CreatedAt,
		UpdatedAt:     row.UpdatedAt,
	}, true, nil
}

// Set stores one private Eino checkpoint payload with safe metadata.
// Set 保存一份私有 Eino checkpoint payload 及其安全元数据。
func (s *PostgresRuntimeStore) Set(ctx context.Context, checkpointID string, payload []byte) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("postgres runtime store is not configured")
	}
	snapshot, err := runtimeGraphCheckpointSnapshot(checkpointID, payload, time.Now().UTC())
	if err != nil {
		return err
	}
	metadata, err := marshalObject(map[string]any{
		"payload_storage":  "opaque_private_runtime_checkpoint",
		"payload_sha256":   snapshot.PayloadSHA256,
		"payload_size":     snapshot.PayloadSize,
		"redaction_policy": "credential_pattern_reject",
	})
	if err != nil {
		return err
	}
	row := postgresRuntimeGraphCheckpointModel{
		CheckpointID:  snapshot.CheckpointID,
		Payload:       append([]byte(nil), payload...),
		PayloadSize:   snapshot.PayloadSize,
		PayloadSHA256: snapshot.PayloadSHA256,
		MetadataJSON:  metadata,
		CreatedAt:     snapshot.CreatedAt,
		UpdatedAt:     snapshot.UpdatedAt,
	}
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "checkpoint_id"}},
		DoUpdates: clause.Assignments(map[string]any{
			"payload":        row.Payload,
			"payload_size":   row.PayloadSize,
			"payload_sha256": row.PayloadSHA256,
			"metadata_json":  row.MetadataJSON,
			"updated_at":     row.UpdatedAt,
		}),
	}).Create(&row).Error
}

type postgresRuntimeContractModel struct {
	ID                       string    `gorm:"column:id;type:text;primaryKey"`
	Name                     string    `gorm:"column:name;type:text;not null;default:''"`
	Version                  string    `gorm:"column:version;type:text;not null;default:''"`
	Status                   string    `gorm:"column:status;type:text;not null;index:idx_runtime_contracts_status"`
	TaskType                 string    `gorm:"column:task_type;type:text;not null;index:idx_runtime_contracts_task_type"`
	InputSchemaJSON          []byte    `gorm:"column:input_schema_json;type:jsonb;not null"`
	ExecutionProfileJSON     []byte    `gorm:"column:execution_profile_json;type:jsonb;not null"`
	ExitPolicyJSON           []byte    `gorm:"column:exit_policy_json;type:jsonb;not null"`
	CapabilityProfileJSON    []byte    `gorm:"column:capability_profile_json;type:jsonb;not null"`
	GovernancePolicyRefsJSON []byte    `gorm:"column:governance_policy_refs_json;type:jsonb;not null"`
	HookBindingsJSON         []byte    `gorm:"column:hook_bindings_json;type:jsonb;not null"`
	ProjectionPolicyJSON     []byte    `gorm:"column:projection_policy_json;type:jsonb;not null"`
	SystemTruthRefsJSON      []byte    `gorm:"column:system_truth_refs_json;type:jsonb;not null"`
	IdempotencyScope         string    `gorm:"column:idempotency_scope;type:text;not null;default:'';index:idx_runtime_contracts_idempotency"`
	IdempotencyKey           string    `gorm:"column:idempotency_key;type:text;not null;default:'';index:idx_runtime_contracts_idempotency"`
	MetadataJSON             []byte    `gorm:"column:metadata_json;type:jsonb;not null"`
	CreatedAt                time.Time `gorm:"column:created_at;type:timestamptz;not null;autoCreateTime;index:idx_runtime_contracts_created_at"`
	UpdatedAt                time.Time `gorm:"column:updated_at;type:timestamptz;not null;autoUpdateTime"`
}

func (postgresRuntimeContractModel) TableName() string { return "runtime_contracts" }

type postgresTaskTypeRegistrationModel struct {
	ID                string    `gorm:"column:id;type:text;primaryKey"`
	TypeKey           string    `gorm:"column:type_key;type:text;not null;uniqueIndex:idx_runtime_task_types_type_key"`
	DisplayName       string    `gorm:"column:display_name;type:text;not null;default:''"`
	Description       string    `gorm:"column:description;type:text;not null;default:''"`
	Status            string    `gorm:"column:status;type:text;not null;index:idx_runtime_task_types_status"`
	InputSchemaJSON   []byte    `gorm:"column:input_schema_json;type:jsonb;not null"`
	ValidatorRefsJSON []byte    `gorm:"column:validator_refs_json;type:jsonb;not null"`
	DefaultContractID string    `gorm:"column:default_contract_id;type:text;not null;default:'';index:idx_runtime_task_types_default_contract"`
	CompatibilityJSON []byte    `gorm:"column:compatibility_json;type:jsonb;not null"`
	MetadataJSON      []byte    `gorm:"column:metadata_json;type:jsonb;not null"`
	CreatedAt         time.Time `gorm:"column:created_at;type:timestamptz;not null;autoCreateTime"`
	UpdatedAt         time.Time `gorm:"column:updated_at;type:timestamptz;not null;autoUpdateTime"`
}

func (postgresTaskTypeRegistrationModel) TableName() string { return "runtime_task_types" }

type postgresHookBindingModel struct {
	ID            string    `gorm:"column:id;type:text;primaryKey"`
	ContractID    string    `gorm:"column:contract_id;type:text;not null;index:idx_runtime_hook_bindings_contract_hook,priority:1"`
	HookPoint     string    `gorm:"column:hook_point;type:text;not null;index:idx_runtime_hook_bindings_contract_hook,priority:2"`
	BindingKind   string    `gorm:"column:binding_kind;type:text;not null;index:idx_runtime_hook_bindings_kind"`
	BindingRef    string    `gorm:"column:binding_ref;type:text;not null;default:''"`
	OrderIndex    int       `gorm:"column:order_index;type:integer;not null;default:0;index:idx_runtime_hook_bindings_order"`
	Enabled       bool      `gorm:"column:enabled;type:boolean;not null;default:true;index:idx_runtime_hook_bindings_enabled"`
	FailurePolicy string    `gorm:"column:failure_policy;type:text;not null;default:''"`
	ConfigJSON    []byte    `gorm:"column:config_json;type:jsonb;not null"`
	MetadataJSON  []byte    `gorm:"column:metadata_json;type:jsonb;not null"`
	CreatedAt     time.Time `gorm:"column:created_at;type:timestamptz;not null;autoCreateTime"`
	UpdatedAt     time.Time `gorm:"column:updated_at;type:timestamptz;not null;autoUpdateTime"`
}

func (postgresHookBindingModel) TableName() string { return "runtime_hook_bindings" }

type postgresSystemTruthSourceModel struct {
	ID           string    `gorm:"column:id;type:text;primaryKey"`
	AssetID      string    `gorm:"column:asset_id;type:text;not null;index:idx_system_truth_sources_asset"`
	SourceKind   string    `gorm:"column:source_kind;type:text;not null;default:''"`
	SourceRef    string    `gorm:"column:source_ref;type:text;not null;default:''"`
	Status       string    `gorm:"column:status;type:text;not null;default:'';index:idx_system_truth_sources_status"`
	ContentJSON  []byte    `gorm:"column:content_json;type:jsonb;not null"`
	ContentHash  string    `gorm:"column:content_hash;type:text;not null;default:'';index:idx_system_truth_sources_hash"`
	MetadataJSON []byte    `gorm:"column:metadata_json;type:jsonb;not null"`
	CreatedAt    time.Time `gorm:"column:created_at;type:timestamptz;not null;autoCreateTime"`
}

func (postgresSystemTruthSourceModel) TableName() string { return "system_truth_sources" }

type postgresSystemTruthDraftModel struct {
	ID           string    `gorm:"column:id;type:text;primaryKey"`
	SourceID     string    `gorm:"column:source_id;type:text;not null;index:idx_system_truth_drafts_source"`
	AssetID      string    `gorm:"column:asset_id;type:text;not null;index:idx_system_truth_drafts_asset"`
	Status       string    `gorm:"column:status;type:text;not null;default:'';index:idx_system_truth_drafts_status"`
	Author       string    `gorm:"column:author;type:text;not null;default:''"`
	Reason       string    `gorm:"column:reason;type:text;not null;default:''"`
	BaseActiveID string    `gorm:"column:base_active_id;type:text;not null;default:''"`
	ContentJSON  []byte    `gorm:"column:content_json;type:jsonb;not null"`
	DiffSummary  string    `gorm:"column:diff_summary;type:text;not null;default:''"`
	MetadataJSON []byte    `gorm:"column:metadata_json;type:jsonb;not null"`
	CreatedAt    time.Time `gorm:"column:created_at;type:timestamptz;not null;autoCreateTime"`
	UpdatedAt    time.Time `gorm:"column:updated_at;type:timestamptz;not null;autoUpdateTime"`
}

func (postgresSystemTruthDraftModel) TableName() string { return "system_truth_drafts" }

type postgresSystemTruthCompileResultModel struct {
	ID                  string    `gorm:"column:id;type:text;primaryKey"`
	DraftID             string    `gorm:"column:draft_id;type:text;not null;index:idx_system_truth_compile_draft"`
	AssetID             string    `gorm:"column:asset_id;type:text;not null;index:idx_system_truth_compile_asset"`
	Status              string    `gorm:"column:status;type:text;not null;default:'';index:idx_system_truth_compile_status"`
	Summary             string    `gorm:"column:summary;type:text;not null;default:''"`
	DiagnosticsJSON     []byte    `gorm:"column:diagnostics_json;type:jsonb;not null"`
	CompiledPayloadJSON []byte    `gorm:"column:compiled_payload_json;type:jsonb;not null"`
	ContentHash         string    `gorm:"column:content_hash;type:text;not null;default:''"`
	MetadataJSON        []byte    `gorm:"column:metadata_json;type:jsonb;not null"`
	CreatedAt           time.Time `gorm:"column:created_at;type:timestamptz;not null;autoCreateTime"`
}

func (postgresSystemTruthCompileResultModel) TableName() string {
	return "system_truth_compile_results"
}

type postgresSystemTruthActiveVersionModel struct {
	ID              string    `gorm:"column:id;type:text;primaryKey"`
	AssetID         string    `gorm:"column:asset_id;type:text;not null;index:idx_system_truth_active_asset"`
	CompileResultID string    `gorm:"column:compile_result_id;type:text;not null;index:idx_system_truth_active_compile"`
	DraftID         string    `gorm:"column:draft_id;type:text;not null;index:idx_system_truth_active_draft"`
	ActivatedBy     string    `gorm:"column:activated_by;type:text;not null;default:''"`
	Reason          string    `gorm:"column:reason;type:text;not null;default:''"`
	RollbackFromID  string    `gorm:"column:rollback_from_id;type:text;not null;default:''"`
	MetadataJSON    []byte    `gorm:"column:metadata_json;type:jsonb;not null"`
	ActivatedAt     time.Time `gorm:"column:activated_at;type:timestamptz;not null;index:idx_system_truth_active_activated_at"`
}

func (postgresSystemTruthActiveVersionModel) TableName() string {
	return "system_truth_active_versions"
}

type postgresTaskRunModel struct {
	ID               string     `gorm:"column:id;type:text;primaryKey"`
	TaskID           string     `gorm:"column:task_id;type:text;not null;index:idx_task_runs_task_id"`
	TaskType         string     `gorm:"column:task_type;type:text;not null;index:idx_task_runs_task_type"`
	TaskSubtype      string     `gorm:"column:task_subtype;type:text;not null;default:''"`
	InputKind        string     `gorm:"column:input_kind;type:text;not null;default:''"`
	Scene            string     `gorm:"column:scene;type:text;not null;default:'';index:idx_task_runs_scene"`
	WorkspaceID      string     `gorm:"column:workspace_id;type:text;not null;default:'';index:idx_task_runs_workspace_status,priority:1"`
	AppInstanceID    string     `gorm:"column:app_instance_id;type:text;not null;default:''"`
	Status           string     `gorm:"column:status;type:text;not null;index:idx_task_runs_workspace_status,priority:2"`
	IdempotencyScope string     `gorm:"column:idempotency_scope;type:text;not null;default:'';index:idx_task_runs_idempotency"`
	IdempotencyKey   string     `gorm:"column:idempotency_key;type:text;not null;default:'';index:idx_task_runs_idempotency"`
	RetentionPolicy  string     `gorm:"column:retention_policy;type:text;not null;default:''"`
	MetadataJSON     []byte     `gorm:"column:metadata_json;type:jsonb;not null"`
	StartedAt        *time.Time `gorm:"column:started_at;type:timestamptz"`
	CompletedAt      *time.Time `gorm:"column:completed_at;type:timestamptz"`
	CreatedAt        time.Time  `gorm:"column:created_at;type:timestamptz;not null;autoCreateTime;index:idx_task_runs_created_at"`
	UpdatedAt        time.Time  `gorm:"column:updated_at;type:timestamptz;not null;autoUpdateTime"`
}

func (postgresTaskRunModel) TableName() string { return "task_runs" }

type postgresTaskStepModel struct {
	ID           string     `gorm:"column:id;type:text;primaryKey"`
	RunID        string     `gorm:"column:run_id;type:text;not null;index:idx_task_steps_run_sequence,priority:1"`
	Sequence     int        `gorm:"column:sequence;type:integer;not null;index:idx_task_steps_run_sequence,priority:2"`
	StepType     string     `gorm:"column:step_type;type:text;not null;default:''"`
	Name         string     `gorm:"column:name;type:text;not null;default:''"`
	Status       string     `gorm:"column:status;type:text;not null;index:idx_task_steps_status"`
	MetadataJSON []byte     `gorm:"column:metadata_json;type:jsonb;not null"`
	StartedAt    *time.Time `gorm:"column:started_at;type:timestamptz"`
	CompletedAt  *time.Time `gorm:"column:completed_at;type:timestamptz"`
	CreatedAt    time.Time  `gorm:"column:created_at;type:timestamptz;not null;autoCreateTime"`
	UpdatedAt    time.Time  `gorm:"column:updated_at;type:timestamptz;not null;autoUpdateTime"`
}

func (postgresTaskStepModel) TableName() string { return "task_steps" }

type postgresRuntimeTraceModel struct {
	ID                  string    `gorm:"column:id;type:text;primaryKey"`
	RunID               string    `gorm:"column:run_id;type:text;not null;index:idx_runtime_traces_run_step,priority:1"`
	StepID              string    `gorm:"column:step_id;type:text;not null;default:'';index:idx_runtime_traces_run_step,priority:2"`
	TraceType           string    `gorm:"column:trace_type;type:text;not null;default:''"`
	Summary             string    `gorm:"column:summary;type:text;not null;default:''"`
	SafeLabelsJSON      []byte    `gorm:"column:safe_labels_json;type:jsonb;not null"`
	RedactedPayloadJSON []byte    `gorm:"column:redacted_payload_json;type:jsonb;not null"`
	MetadataJSON        []byte    `gorm:"column:metadata_json;type:jsonb;not null"`
	CreatedAt           time.Time `gorm:"column:created_at;type:timestamptz;not null;autoCreateTime"`
}

func (postgresRuntimeTraceModel) TableName() string { return "runtime_traces" }

type postgresUsageModel struct {
	ID           string    `gorm:"column:id;type:text;primaryKey"`
	RunID        string    `gorm:"column:run_id;type:text;not null;index:idx_runtime_usage_run_step,priority:1"`
	StepID       string    `gorm:"column:step_id;type:text;not null;default:'';index:idx_runtime_usage_run_step,priority:2"`
	ResourceType string    `gorm:"column:resource_type;type:text;not null;index:idx_runtime_usage_resource"`
	Provider     string    `gorm:"column:provider;type:text;not null;default:''"`
	ResourceName string    `gorm:"column:resource_name;type:text;not null;default:''"`
	Unit         string    `gorm:"column:unit;type:text;not null;default:''"`
	Amount       float64   `gorm:"column:amount;type:numeric;not null;default:0"`
	Cost         *float64  `gorm:"column:cost;type:numeric"`
	Currency     string    `gorm:"column:currency;type:text;not null;default:''"`
	MetadataJSON []byte    `gorm:"column:metadata_json;type:jsonb;not null"`
	CreatedAt    time.Time `gorm:"column:created_at;type:timestamptz;not null;autoCreateTime"`
}

func (postgresUsageModel) TableName() string { return "runtime_usage" }

type postgresLifecycleEventModel struct {
	ID           string    `gorm:"column:id;type:text;primaryKey"`
	RunID        string    `gorm:"column:run_id;type:text;not null;index:idx_runtime_lifecycle_run"`
	StepID       string    `gorm:"column:step_id;type:text;not null;default:'';index:idx_runtime_lifecycle_step"`
	EventType    string    `gorm:"column:event_type;type:text;not null;index:idx_runtime_lifecycle_event_type"`
	SubjectType  string    `gorm:"column:subject_type;type:text;not null;index:idx_runtime_lifecycle_subject,priority:1"`
	SubjectID    string    `gorm:"column:subject_id;type:text;not null;index:idx_runtime_lifecycle_subject,priority:2"`
	FromStatus   string    `gorm:"column:from_status;type:text;not null;default:''"`
	ToStatus     string    `gorm:"column:to_status;type:text;not null;default:''"`
	Reason       string    `gorm:"column:reason;type:text;not null;default:''"`
	MetadataJSON []byte    `gorm:"column:metadata_json;type:jsonb;not null"`
	OccurredAt   time.Time `gorm:"column:occurred_at;type:timestamptz;not null;index:idx_runtime_lifecycle_occurred_at"`
}

func (postgresLifecycleEventModel) TableName() string { return "task_run_lifecycle_events" }

type postgresProjectionCandidateModel struct {
	ID                        string    `gorm:"column:id;type:text;primaryKey"`
	RunID                     string    `gorm:"column:run_id;type:text;not null;index:idx_runtime_projections_run_step,priority:1"`
	StepID                    string    `gorm:"column:step_id;type:text;not null;default:'';index:idx_runtime_projections_run_step,priority:2"`
	CandidateKind             string    `gorm:"column:candidate_kind;type:text;not null;default:''"`
	Status                    string    `gorm:"column:status;type:text;not null;default:''"`
	Summary                   string    `gorm:"column:summary;type:text;not null;default:''"`
	SchemaVersion             string    `gorm:"column:schema_version;type:text;not null;default:''"`
	RedactedPayloadJSON       []byte    `gorm:"column:redacted_payload_json;type:jsonb;not null"`
	SemanticPayloadJSON       []byte    `gorm:"column:semantic_payload_json;type:jsonb;not null;default:'{}'"`
	ArtifactRefsJSON          []byte    `gorm:"column:artifact_refs_json;type:jsonb;not null;default:'{}'"`
	UIHintsJSON               []byte    `gorm:"column:ui_hints_json;type:jsonb;not null;default:'{}'"`
	MaterializationTargetJSON []byte    `gorm:"column:materialization_target_json;type:jsonb;not null;default:'{}'"`
	MetadataJSON              []byte    `gorm:"column:metadata_json;type:jsonb;not null"`
	CreatedAt                 time.Time `gorm:"column:created_at;type:timestamptz;not null;autoCreateTime"`
}

func (postgresProjectionCandidateModel) TableName() string { return "runtime_projections" }

type postgresRuntimeGraphCheckpointModel struct {
	CheckpointID  string    `gorm:"column:checkpoint_id;type:text;primaryKey"`
	Payload       []byte    `gorm:"column:payload;type:bytea;not null"`
	PayloadSize   int       `gorm:"column:payload_size;type:integer;not null;default:0"`
	PayloadSHA256 string    `gorm:"column:payload_sha256;type:text;not null;default:'';index:idx_runtime_graph_checkpoints_sha"`
	MetadataJSON  []byte    `gorm:"column:metadata_json;type:jsonb;not null"`
	CreatedAt     time.Time `gorm:"column:created_at;type:timestamptz;not null;autoCreateTime"`
	UpdatedAt     time.Time `gorm:"column:updated_at;type:timestamptz;not null;autoUpdateTime"`
}

func (postgresRuntimeGraphCheckpointModel) TableName() string { return "runtime_graph_checkpoints" }

func taskTypeRegistrationToRow(input TaskTypeRegistration) (postgresTaskTypeRegistrationModel, error) {
	inputSchema, err := marshalObject(input.InputSchema)
	if err != nil {
		return postgresTaskTypeRegistrationModel{}, err
	}
	validatorRefs, err := marshalObject(input.ValidatorRefs)
	if err != nil {
		return postgresTaskTypeRegistrationModel{}, err
	}
	compatibility, err := marshalObject(input.Compatibility)
	if err != nil {
		return postgresTaskTypeRegistrationModel{}, err
	}
	metadata, err := marshalObject(input.Metadata)
	if err != nil {
		return postgresTaskTypeRegistrationModel{}, err
	}
	return postgresTaskTypeRegistrationModel{
		ID:                defaultID(input.ID),
		TypeKey:           strings.TrimSpace(input.TypeKey),
		DisplayName:       strings.TrimSpace(input.DisplayName),
		Description:       strings.TrimSpace(input.Description),
		Status:            defaultString(input.Status, TaskTypeStatusDraft),
		InputSchemaJSON:   inputSchema,
		ValidatorRefsJSON: validatorRefs,
		DefaultContractID: strings.TrimSpace(input.DefaultContractID),
		CompatibilityJSON: compatibility,
		MetadataJSON:      metadata,
		CreatedAt:         input.CreatedAt,
		UpdatedAt:         input.UpdatedAt,
	}, nil
}

func taskTypeRegistrationFromRow(row postgresTaskTypeRegistrationModel) (TaskTypeRegistration, error) {
	inputSchema, err := unmarshalAnyMap(row.InputSchemaJSON)
	if err != nil {
		return TaskTypeRegistration{}, err
	}
	validatorRefs, err := unmarshalAnyMap(row.ValidatorRefsJSON)
	if err != nil {
		return TaskTypeRegistration{}, err
	}
	compatibility, err := unmarshalAnyMap(row.CompatibilityJSON)
	if err != nil {
		return TaskTypeRegistration{}, err
	}
	metadata, err := unmarshalAnyMap(row.MetadataJSON)
	if err != nil {
		return TaskTypeRegistration{}, err
	}
	return TaskTypeRegistration{ID: row.ID, TypeKey: row.TypeKey, DisplayName: row.DisplayName, Description: row.Description, Status: row.Status, InputSchema: inputSchema, ValidatorRefs: validatorRefs, DefaultContractID: row.DefaultContractID, Compatibility: compatibility, Metadata: metadata, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}, nil
}

func taskTypeRegistrationsFromRows(rows []postgresTaskTypeRegistrationModel) ([]TaskTypeRegistration, error) {
	output := make([]TaskTypeRegistration, 0, len(rows))
	for _, row := range rows {
		item, err := taskTypeRegistrationFromRow(row)
		if err != nil {
			return nil, err
		}
		output = append(output, item)
	}
	return output, nil
}

func hookBindingToRow(input HookBinding) (postgresHookBindingModel, error) {
	config, err := marshalObject(input.Config)
	if err != nil {
		return postgresHookBindingModel{}, err
	}
	metadata, err := marshalObject(input.Metadata)
	if err != nil {
		return postgresHookBindingModel{}, err
	}
	return postgresHookBindingModel{ID: defaultID(input.ID), ContractID: strings.TrimSpace(input.ContractID), HookPoint: strings.TrimSpace(input.HookPoint), BindingKind: strings.TrimSpace(input.BindingKind), BindingRef: strings.TrimSpace(input.BindingRef), OrderIndex: input.OrderIndex, Enabled: input.Enabled, FailurePolicy: defaultString(input.FailurePolicy, HookFailurePolicyFailClosed), ConfigJSON: config, MetadataJSON: metadata, CreatedAt: input.CreatedAt, UpdatedAt: input.UpdatedAt}, nil
}

func hookBindingFromRow(row postgresHookBindingModel) (HookBinding, error) {
	config, err := unmarshalAnyMap(row.ConfigJSON)
	if err != nil {
		return HookBinding{}, err
	}
	metadata, err := unmarshalAnyMap(row.MetadataJSON)
	if err != nil {
		return HookBinding{}, err
	}
	return HookBinding{ID: row.ID, ContractID: row.ContractID, HookPoint: row.HookPoint, BindingKind: row.BindingKind, BindingRef: row.BindingRef, OrderIndex: row.OrderIndex, Enabled: row.Enabled, FailurePolicy: row.FailurePolicy, Config: config, Metadata: metadata, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}, nil
}

func hookBindingsFromRows(rows []postgresHookBindingModel) ([]HookBinding, error) {
	output := make([]HookBinding, 0, len(rows))
	for _, row := range rows {
		item, err := hookBindingFromRow(row)
		if err != nil {
			return nil, err
		}
		output = append(output, item)
	}
	return output, nil
}

func systemTruthSourceToRow(input SystemTruthSource) (postgresSystemTruthSourceModel, error) {
	content, err := marshalObject(input.Content)
	if err != nil {
		return postgresSystemTruthSourceModel{}, err
	}
	metadata, err := marshalObject(input.Metadata)
	if err != nil {
		return postgresSystemTruthSourceModel{}, err
	}
	return postgresSystemTruthSourceModel{ID: defaultID(input.ID), AssetID: strings.TrimSpace(input.AssetID), SourceKind: strings.TrimSpace(input.SourceKind), SourceRef: strings.TrimSpace(input.SourceRef), Status: defaultString(input.Status, SystemTruthSourceStatusImported), ContentJSON: content, ContentHash: strings.TrimSpace(input.ContentHash), MetadataJSON: metadata, CreatedAt: input.CreatedAt}, nil
}

func systemTruthSourceFromRow(row postgresSystemTruthSourceModel) (SystemTruthSource, error) {
	content, err := unmarshalAnyMap(row.ContentJSON)
	if err != nil {
		return SystemTruthSource{}, err
	}
	metadata, err := unmarshalAnyMap(row.MetadataJSON)
	if err != nil {
		return SystemTruthSource{}, err
	}
	return SystemTruthSource{ID: row.ID, AssetID: row.AssetID, SourceKind: row.SourceKind, SourceRef: row.SourceRef, Status: row.Status, Content: content, ContentHash: row.ContentHash, Metadata: metadata, CreatedAt: row.CreatedAt}, nil
}

func systemTruthDraftToRow(input SystemTruthDraft) (postgresSystemTruthDraftModel, error) {
	content, err := marshalObject(input.Content)
	if err != nil {
		return postgresSystemTruthDraftModel{}, err
	}
	metadata, err := marshalObject(input.Metadata)
	if err != nil {
		return postgresSystemTruthDraftModel{}, err
	}
	return postgresSystemTruthDraftModel{ID: defaultID(input.ID), SourceID: strings.TrimSpace(input.SourceID), AssetID: strings.TrimSpace(input.AssetID), Status: defaultString(input.Status, SystemTruthDraftStatusDraft), Author: strings.TrimSpace(input.Author), Reason: strings.TrimSpace(input.Reason), BaseActiveID: strings.TrimSpace(input.BaseActiveID), ContentJSON: content, DiffSummary: strings.TrimSpace(input.DiffSummary), MetadataJSON: metadata, CreatedAt: input.CreatedAt, UpdatedAt: input.UpdatedAt}, nil
}

func systemTruthDraftFromRow(row postgresSystemTruthDraftModel) (SystemTruthDraft, error) {
	content, err := unmarshalAnyMap(row.ContentJSON)
	if err != nil {
		return SystemTruthDraft{}, err
	}
	metadata, err := unmarshalAnyMap(row.MetadataJSON)
	if err != nil {
		return SystemTruthDraft{}, err
	}
	return SystemTruthDraft{ID: row.ID, SourceID: row.SourceID, AssetID: row.AssetID, Status: row.Status, Author: row.Author, Reason: row.Reason, BaseActiveID: row.BaseActiveID, Content: content, DiffSummary: row.DiffSummary, Metadata: metadata, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}, nil
}

func systemTruthCompileResultToRow(input SystemTruthCompileResult) (postgresSystemTruthCompileResultModel, error) {
	diagnostics, err := marshalObject(input.Diagnostics)
	if err != nil {
		return postgresSystemTruthCompileResultModel{}, err
	}
	payload, err := marshalObject(input.CompiledPayload)
	if err != nil {
		return postgresSystemTruthCompileResultModel{}, err
	}
	metadata, err := marshalObject(input.Metadata)
	if err != nil {
		return postgresSystemTruthCompileResultModel{}, err
	}
	return postgresSystemTruthCompileResultModel{ID: defaultID(input.ID), DraftID: strings.TrimSpace(input.DraftID), AssetID: strings.TrimSpace(input.AssetID), Status: strings.TrimSpace(input.Status), Summary: strings.TrimSpace(input.Summary), DiagnosticsJSON: diagnostics, CompiledPayloadJSON: payload, ContentHash: strings.TrimSpace(input.ContentHash), MetadataJSON: metadata, CreatedAt: input.CreatedAt}, nil
}

func systemTruthCompileResultFromRow(row postgresSystemTruthCompileResultModel) (SystemTruthCompileResult, error) {
	diagnostics, err := unmarshalAnyMap(row.DiagnosticsJSON)
	if err != nil {
		return SystemTruthCompileResult{}, err
	}
	payload, err := unmarshalAnyMap(row.CompiledPayloadJSON)
	if err != nil {
		return SystemTruthCompileResult{}, err
	}
	metadata, err := unmarshalAnyMap(row.MetadataJSON)
	if err != nil {
		return SystemTruthCompileResult{}, err
	}
	return SystemTruthCompileResult{ID: row.ID, DraftID: row.DraftID, AssetID: row.AssetID, Status: row.Status, Summary: row.Summary, Diagnostics: diagnostics, CompiledPayload: payload, ContentHash: row.ContentHash, Metadata: metadata, CreatedAt: row.CreatedAt}, nil
}

func systemTruthActiveVersionToRow(input SystemTruthActiveVersion) (postgresSystemTruthActiveVersionModel, error) {
	metadata, err := marshalObject(input.Metadata)
	if err != nil {
		return postgresSystemTruthActiveVersionModel{}, err
	}
	activatedAt := input.ActivatedAt
	if activatedAt.IsZero() {
		activatedAt = time.Now().UTC()
	}
	return postgresSystemTruthActiveVersionModel{ID: defaultID(input.ID), AssetID: strings.TrimSpace(input.AssetID), CompileResultID: strings.TrimSpace(input.CompileResultID), DraftID: strings.TrimSpace(input.DraftID), ActivatedBy: strings.TrimSpace(input.ActivatedBy), Reason: strings.TrimSpace(input.Reason), RollbackFromID: strings.TrimSpace(input.RollbackFromID), MetadataJSON: metadata, ActivatedAt: activatedAt}, nil
}

func systemTruthActiveVersionFromRow(row postgresSystemTruthActiveVersionModel) (SystemTruthActiveVersion, error) {
	metadata, err := unmarshalAnyMap(row.MetadataJSON)
	if err != nil {
		return SystemTruthActiveVersion{}, err
	}
	return SystemTruthActiveVersion{ID: row.ID, AssetID: row.AssetID, CompileResultID: row.CompileResultID, DraftID: row.DraftID, ActivatedBy: row.ActivatedBy, Reason: row.Reason, RollbackFromID: row.RollbackFromID, Metadata: metadata, ActivatedAt: row.ActivatedAt}, nil
}

func systemTruthActiveVersionsFromRows(rows []postgresSystemTruthActiveVersionModel) ([]SystemTruthActiveVersion, error) {
	output := make([]SystemTruthActiveVersion, 0, len(rows))
	for _, row := range rows {
		item, err := systemTruthActiveVersionFromRow(row)
		if err != nil {
			return nil, err
		}
		output = append(output, item)
	}
	return output, nil
}

func runtimeContractToRow(input RuntimeContract) (postgresRuntimeContractModel, error) {
	inputSchema, err := marshalObject(input.InputSchema)
	if err != nil {
		return postgresRuntimeContractModel{}, err
	}
	executionProfile, err := marshalObject(input.ExecutionProfile)
	if err != nil {
		return postgresRuntimeContractModel{}, err
	}
	exitPolicy, err := marshalObject(input.ExitPolicy)
	if err != nil {
		return postgresRuntimeContractModel{}, err
	}
	capabilityProfile, err := marshalObject(input.CapabilityProfile)
	if err != nil {
		return postgresRuntimeContractModel{}, err
	}
	governancePolicyRefs, err := marshalObject(input.GovernancePolicyRefs)
	if err != nil {
		return postgresRuntimeContractModel{}, err
	}
	hookBindings, err := marshalObject(input.HookBindings)
	if err != nil {
		return postgresRuntimeContractModel{}, err
	}
	projectionPolicy, err := marshalObject(input.ProjectionPolicy)
	if err != nil {
		return postgresRuntimeContractModel{}, err
	}
	systemTruthRefs, err := marshalObject(input.SystemTruthRefs)
	if err != nil {
		return postgresRuntimeContractModel{}, err
	}
	metadata, err := marshalObject(input.Metadata)
	if err != nil {
		return postgresRuntimeContractModel{}, err
	}
	return postgresRuntimeContractModel{
		ID:                       defaultID(input.ID),
		Name:                     strings.TrimSpace(input.Name),
		Version:                  strings.TrimSpace(input.Version),
		Status:                   defaultString(input.Status, RuntimeContractStatusDraft),
		TaskType:                 strings.TrimSpace(input.TaskType),
		InputSchemaJSON:          inputSchema,
		ExecutionProfileJSON:     executionProfile,
		ExitPolicyJSON:           exitPolicy,
		CapabilityProfileJSON:    capabilityProfile,
		GovernancePolicyRefsJSON: governancePolicyRefs,
		HookBindingsJSON:         hookBindings,
		ProjectionPolicyJSON:     projectionPolicy,
		SystemTruthRefsJSON:      systemTruthRefs,
		IdempotencyScope:         strings.TrimSpace(input.IdempotencyScope),
		IdempotencyKey:           strings.TrimSpace(input.IdempotencyKey),
		MetadataJSON:             metadata,
		CreatedAt:                input.CreatedAt,
		UpdatedAt:                input.UpdatedAt,
	}, nil
}

func runtimeContractFromRow(row postgresRuntimeContractModel) (RuntimeContract, error) {
	inputSchema, err := unmarshalAnyMap(row.InputSchemaJSON)
	if err != nil {
		return RuntimeContract{}, err
	}
	executionProfile, err := unmarshalAnyMap(row.ExecutionProfileJSON)
	if err != nil {
		return RuntimeContract{}, err
	}
	exitPolicy, err := unmarshalAnyMap(row.ExitPolicyJSON)
	if err != nil {
		return RuntimeContract{}, err
	}
	capabilityProfile, err := unmarshalAnyMap(row.CapabilityProfileJSON)
	if err != nil {
		return RuntimeContract{}, err
	}
	governancePolicyRefs, err := unmarshalAnyMap(row.GovernancePolicyRefsJSON)
	if err != nil {
		return RuntimeContract{}, err
	}
	hookBindings, err := unmarshalAnyMap(row.HookBindingsJSON)
	if err != nil {
		return RuntimeContract{}, err
	}
	projectionPolicy, err := unmarshalAnyMap(row.ProjectionPolicyJSON)
	if err != nil {
		return RuntimeContract{}, err
	}
	systemTruthRefs, err := unmarshalAnyMap(row.SystemTruthRefsJSON)
	if err != nil {
		return RuntimeContract{}, err
	}
	metadata, err := unmarshalAnyMap(row.MetadataJSON)
	if err != nil {
		return RuntimeContract{}, err
	}
	return RuntimeContract{
		ID:                   row.ID,
		Name:                 row.Name,
		Version:              row.Version,
		Status:               row.Status,
		TaskType:             row.TaskType,
		InputSchema:          inputSchema,
		ExecutionProfile:     executionProfile,
		ExitPolicy:           exitPolicy,
		CapabilityProfile:    capabilityProfile,
		GovernancePolicyRefs: governancePolicyRefs,
		HookBindings:         hookBindings,
		ProjectionPolicy:     projectionPolicy,
		SystemTruthRefs:      systemTruthRefs,
		IdempotencyScope:     row.IdempotencyScope,
		IdempotencyKey:       row.IdempotencyKey,
		Metadata:             metadata,
		CreatedAt:            row.CreatedAt,
		UpdatedAt:            row.UpdatedAt,
	}, nil
}

func runtimeContractsFromRows(rows []postgresRuntimeContractModel) ([]RuntimeContract, error) {
	output := make([]RuntimeContract, 0, len(rows))
	for _, row := range rows {
		item, err := runtimeContractFromRow(row)
		if err != nil {
			return nil, err
		}
		output = append(output, item)
	}
	return output, nil
}

func taskRunToRow(input TaskRun) (postgresTaskRunModel, error) {
	metadata, err := marshalObject(input.Metadata)
	if err != nil {
		return postgresTaskRunModel{}, err
	}
	return postgresTaskRunModel{
		ID:               defaultID(input.ID),
		TaskID:           strings.TrimSpace(input.TaskID),
		TaskType:         defaultString(input.TaskType, "runtime_task"),
		TaskSubtype:      strings.TrimSpace(input.TaskSubtype),
		InputKind:        strings.TrimSpace(input.InputKind),
		Scene:            strings.TrimSpace(input.Scene),
		WorkspaceID:      strings.TrimSpace(input.WorkspaceID),
		AppInstanceID:    strings.TrimSpace(input.AppInstanceID),
		Status:           defaultString(input.Status, TaskRunStatusCreated),
		IdempotencyScope: strings.TrimSpace(input.IdempotencyScope),
		IdempotencyKey:   strings.TrimSpace(input.IdempotencyKey),
		RetentionPolicy:  strings.TrimSpace(input.RetentionPolicy),
		MetadataJSON:     metadata,
		StartedAt:        input.StartedAt,
		CompletedAt:      input.CompletedAt,
		CreatedAt:        input.CreatedAt,
		UpdatedAt:        input.UpdatedAt,
	}, nil
}

func taskRunFromRow(row postgresTaskRunModel) (TaskRun, error) {
	metadata, err := unmarshalAnyMap(row.MetadataJSON)
	if err != nil {
		return TaskRun{}, err
	}
	return TaskRun{ID: row.ID, TaskID: row.TaskID, TaskType: row.TaskType, TaskSubtype: row.TaskSubtype, InputKind: row.InputKind, Scene: row.Scene, WorkspaceID: row.WorkspaceID, AppInstanceID: row.AppInstanceID, Status: row.Status, IdempotencyScope: row.IdempotencyScope, IdempotencyKey: row.IdempotencyKey, RetentionPolicy: row.RetentionPolicy, Metadata: metadata, StartedAt: row.StartedAt, CompletedAt: row.CompletedAt, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}, nil
}

func taskRunsFromRows(rows []postgresTaskRunModel) ([]TaskRun, error) {
	output := make([]TaskRun, 0, len(rows))
	for _, row := range rows {
		item, err := taskRunFromRow(row)
		if err != nil {
			return nil, err
		}
		output = append(output, item)
	}
	return output, nil
}

func taskStepToRow(input TaskStep) (postgresTaskStepModel, error) {
	metadata, err := marshalObject(input.Metadata)
	if err != nil {
		return postgresTaskStepModel{}, err
	}
	return postgresTaskStepModel{ID: defaultID(input.ID), RunID: strings.TrimSpace(input.RunID), Sequence: input.Sequence, StepType: strings.TrimSpace(input.StepType), Name: strings.TrimSpace(input.Name), Status: defaultString(input.Status, TaskStepStatusPending), MetadataJSON: metadata, StartedAt: input.StartedAt, CompletedAt: input.CompletedAt, CreatedAt: input.CreatedAt, UpdatedAt: input.UpdatedAt}, nil
}

func taskStepFromRow(row postgresTaskStepModel) (TaskStep, error) {
	metadata, err := unmarshalAnyMap(row.MetadataJSON)
	if err != nil {
		return TaskStep{}, err
	}
	return TaskStep{ID: row.ID, RunID: row.RunID, Sequence: row.Sequence, StepType: row.StepType, Name: row.Name, Status: row.Status, Metadata: metadata, StartedAt: row.StartedAt, CompletedAt: row.CompletedAt, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}, nil
}

func taskStepsFromRows(rows []postgresTaskStepModel) ([]TaskStep, error) {
	output := make([]TaskStep, 0, len(rows))
	for _, row := range rows {
		item, err := taskStepFromRow(row)
		if err != nil {
			return nil, err
		}
		output = append(output, item)
	}
	return output, nil
}

func runtimeTraceToRow(input RuntimeTrace) (postgresRuntimeTraceModel, error) {
	labels, err := marshalObject(input.SafeLabels)
	if err != nil {
		return postgresRuntimeTraceModel{}, err
	}
	payload, err := marshalObject(input.RedactedPayload)
	if err != nil {
		return postgresRuntimeTraceModel{}, err
	}
	metadata, err := marshalObject(input.Metadata)
	if err != nil {
		return postgresRuntimeTraceModel{}, err
	}
	return postgresRuntimeTraceModel{ID: defaultID(input.ID), RunID: strings.TrimSpace(input.RunID), StepID: strings.TrimSpace(input.StepID), TraceType: strings.TrimSpace(input.TraceType), Summary: strings.TrimSpace(input.Summary), SafeLabelsJSON: labels, RedactedPayloadJSON: payload, MetadataJSON: metadata, CreatedAt: input.CreatedAt}, nil
}

func runtimeTraceFromRow(row postgresRuntimeTraceModel) (RuntimeTrace, error) {
	labels, err := unmarshalStringMap(row.SafeLabelsJSON)
	if err != nil {
		return RuntimeTrace{}, err
	}
	payload, err := unmarshalAnyMap(row.RedactedPayloadJSON)
	if err != nil {
		return RuntimeTrace{}, err
	}
	metadata, err := unmarshalAnyMap(row.MetadataJSON)
	if err != nil {
		return RuntimeTrace{}, err
	}
	return RuntimeTrace{ID: row.ID, RunID: row.RunID, StepID: row.StepID, TraceType: row.TraceType, Summary: row.Summary, SafeLabels: labels, RedactedPayload: payload, Metadata: metadata, CreatedAt: row.CreatedAt}, nil
}

func runtimeTracesFromRows(rows []postgresRuntimeTraceModel) ([]RuntimeTrace, error) {
	output := make([]RuntimeTrace, 0, len(rows))
	for _, row := range rows {
		item, err := runtimeTraceFromRow(row)
		if err != nil {
			return nil, err
		}
		output = append(output, item)
	}
	return output, nil
}

func usageToRow(input Usage) (postgresUsageModel, error) {
	metadata, err := marshalObject(input.Metadata)
	if err != nil {
		return postgresUsageModel{}, err
	}
	return postgresUsageModel{ID: defaultID(input.ID), RunID: strings.TrimSpace(input.RunID), StepID: strings.TrimSpace(input.StepID), ResourceType: strings.TrimSpace(input.ResourceType), Provider: strings.TrimSpace(input.Provider), ResourceName: strings.TrimSpace(input.ResourceName), Unit: strings.TrimSpace(input.Unit), Amount: input.Amount, Cost: input.Cost, Currency: strings.TrimSpace(input.Currency), MetadataJSON: metadata, CreatedAt: input.CreatedAt}, nil
}

func usageFromRow(row postgresUsageModel) (Usage, error) {
	metadata, err := unmarshalAnyMap(row.MetadataJSON)
	if err != nil {
		return Usage{}, err
	}
	return Usage{ID: row.ID, RunID: row.RunID, StepID: row.StepID, ResourceType: row.ResourceType, Provider: row.Provider, ResourceName: row.ResourceName, Unit: row.Unit, Amount: row.Amount, Cost: row.Cost, Currency: row.Currency, Metadata: metadata, CreatedAt: row.CreatedAt}, nil
}

func usagesFromRows(rows []postgresUsageModel) ([]Usage, error) {
	output := make([]Usage, 0, len(rows))
	for _, row := range rows {
		item, err := usageFromRow(row)
		if err != nil {
			return nil, err
		}
		output = append(output, item)
	}
	return output, nil
}

func lifecycleEventToRow(input TaskRunLifecycleEvent) (postgresLifecycleEventModel, error) {
	metadata, err := marshalObject(input.Metadata)
	if err != nil {
		return postgresLifecycleEventModel{}, err
	}
	occurredAt := input.OccurredAt
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}
	return postgresLifecycleEventModel{ID: defaultID(input.ID), RunID: strings.TrimSpace(input.RunID), StepID: strings.TrimSpace(input.StepID), EventType: strings.TrimSpace(input.EventType), SubjectType: strings.TrimSpace(input.SubjectType), SubjectID: strings.TrimSpace(input.SubjectID), FromStatus: strings.TrimSpace(input.FromStatus), ToStatus: strings.TrimSpace(input.ToStatus), Reason: strings.TrimSpace(input.Reason), MetadataJSON: metadata, OccurredAt: occurredAt}, nil
}

func lifecycleEventFromRow(row postgresLifecycleEventModel) (TaskRunLifecycleEvent, error) {
	metadata, err := unmarshalAnyMap(row.MetadataJSON)
	if err != nil {
		return TaskRunLifecycleEvent{}, err
	}
	return TaskRunLifecycleEvent{ID: row.ID, RunID: row.RunID, StepID: row.StepID, EventType: row.EventType, SubjectType: row.SubjectType, SubjectID: row.SubjectID, FromStatus: row.FromStatus, ToStatus: row.ToStatus, Reason: row.Reason, Metadata: metadata, OccurredAt: row.OccurredAt}, nil
}

func lifecycleEventsFromRows(rows []postgresLifecycleEventModel) ([]TaskRunLifecycleEvent, error) {
	output := make([]TaskRunLifecycleEvent, 0, len(rows))
	for _, row := range rows {
		item, err := lifecycleEventFromRow(row)
		if err != nil {
			return nil, err
		}
		output = append(output, item)
	}
	return output, nil
}

func projectionCandidateToRow(input ProjectionCandidate) (postgresProjectionCandidateModel, error) {
	payload, err := marshalObject(input.RedactedPayload)
	if err != nil {
		return postgresProjectionCandidateModel{}, err
	}
	semanticPayload, err := marshalObject(input.SemanticPayload)
	if err != nil {
		return postgresProjectionCandidateModel{}, err
	}
	artifactRefs, err := marshalObject(input.ArtifactRefs)
	if err != nil {
		return postgresProjectionCandidateModel{}, err
	}
	uiHints, err := marshalObject(input.UIHints)
	if err != nil {
		return postgresProjectionCandidateModel{}, err
	}
	materializationTarget, err := marshalObject(input.MaterializationTarget)
	if err != nil {
		return postgresProjectionCandidateModel{}, err
	}
	metadata, err := marshalObject(input.Metadata)
	if err != nil {
		return postgresProjectionCandidateModel{}, err
	}
	return postgresProjectionCandidateModel{ID: defaultID(input.ID), RunID: strings.TrimSpace(input.RunID), StepID: strings.TrimSpace(input.StepID), CandidateKind: strings.TrimSpace(input.CandidateKind), Status: strings.TrimSpace(input.Status), Summary: strings.TrimSpace(input.Summary), SchemaVersion: strings.TrimSpace(input.SchemaVersion), RedactedPayloadJSON: payload, SemanticPayloadJSON: semanticPayload, ArtifactRefsJSON: artifactRefs, UIHintsJSON: uiHints, MaterializationTargetJSON: materializationTarget, MetadataJSON: metadata, CreatedAt: input.CreatedAt}, nil
}

func projectionCandidateFromRow(row postgresProjectionCandidateModel) (ProjectionCandidate, error) {
	payload, err := unmarshalAnyMap(row.RedactedPayloadJSON)
	if err != nil {
		return ProjectionCandidate{}, err
	}
	semanticPayload, err := unmarshalAnyMap(row.SemanticPayloadJSON)
	if err != nil {
		return ProjectionCandidate{}, err
	}
	artifactRefs, err := unmarshalAnyMap(row.ArtifactRefsJSON)
	if err != nil {
		return ProjectionCandidate{}, err
	}
	uiHints, err := unmarshalAnyMap(row.UIHintsJSON)
	if err != nil {
		return ProjectionCandidate{}, err
	}
	materializationTarget, err := unmarshalAnyMap(row.MaterializationTargetJSON)
	if err != nil {
		return ProjectionCandidate{}, err
	}
	metadata, err := unmarshalAnyMap(row.MetadataJSON)
	if err != nil {
		return ProjectionCandidate{}, err
	}
	return ProjectionCandidate{ID: row.ID, RunID: row.RunID, StepID: row.StepID, CandidateKind: row.CandidateKind, Status: row.Status, Summary: row.Summary, SchemaVersion: row.SchemaVersion, RedactedPayload: payload, SemanticPayload: semanticPayload, ArtifactRefs: artifactRefs, UIHints: uiHints, MaterializationTarget: materializationTarget, Metadata: metadata, CreatedAt: row.CreatedAt}, nil
}

func projectionCandidatesFromRows(rows []postgresProjectionCandidateModel) ([]ProjectionCandidate, error) {
	output := make([]ProjectionCandidate, 0, len(rows))
	for _, row := range rows {
		item, err := projectionCandidateFromRow(row)
		if err != nil {
			return nil, err
		}
		output = append(output, item)
	}
	return output, nil
}

func marshalObject(value any) ([]byte, error) {
	if value == nil {
		return []byte("{}"), nil
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 || string(raw) == "null" {
		return []byte("{}"), nil
	}
	return raw, nil
}

func unmarshalAnyMap(raw []byte) (map[string]any, error) {
	if len(raw) == 0 {
		return map[string]any{}, nil
	}
	var output map[string]any
	if err := json.Unmarshal(raw, &output); err != nil {
		return nil, err
	}
	if output == nil {
		output = map[string]any{}
	}
	return output, nil
}

func unmarshalStringMap(raw []byte) (map[string]string, error) {
	if len(raw) == 0 {
		return map[string]string{}, nil
	}
	var output map[string]string
	if err := json.Unmarshal(raw, &output); err != nil {
		return nil, err
	}
	if output == nil {
		output = map[string]string{}
	}
	return output, nil
}

func defaultID(value string) string {
	if strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	return uuid.NewString()
}

func defaultString(value string, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	return fallback
}
