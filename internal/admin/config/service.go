package config

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"go.uber.org/zap"

	"rabc-go/api/apiv1"
	"rabc-go/internal/model"
	"rabc-go/pkg/log"
)

func NewService(logger *log.Logger, repo *Repo) *Service {
	return &Service{logger: logger, repo: repo}
}

type Service struct {
	logger *log.Logger
	repo   *Repo
}

// GetString / GetBool / GetInt 是配置的强类型读取入口，供后端其他模块按 key 取值，
// 把弱类型的字符串解析收敛在一处（DRY）。
//
// 当前直接查库：尚无高频消费方，刻意不加内存缓存（YAGNI），也因此天然无多实例
// 一致性问题。将来出现热路径消费方，再引入带失效机制的缓存。
func (s *Service) GetString(ctx context.Context, key string) (string, error) {
	cfg, err := s.repo.GetByKey(ctx, key)
	if err != nil {
		return "", err
	}
	return cfg.ConfigValue, nil
}

func (s *Service) GetBool(ctx context.Context, key string) (bool, error) {
	cfg, err := s.repo.GetByKey(ctx, key)
	if err != nil {
		return false, err
	}
	if cfg.ValueType != model.ConfigTypeBool {
		return false, fmt.Errorf("%w: %s 声明类型为 %s，非 bool", apiv1.ErrConfigInvalidValue, key, cfg.ValueType)
	}
	b, err := strconv.ParseBool(cfg.ConfigValue)
	if err != nil {
		return false, fmt.Errorf("%w: %w", apiv1.ErrConfigInvalidValue, err)
	}
	return b, nil
}

func (s *Service) GetInt(ctx context.Context, key string) (int, error) {
	cfg, err := s.repo.GetByKey(ctx, key)
	if err != nil {
		return 0, err
	}
	if cfg.ValueType != model.ConfigTypeInt {
		return 0, fmt.Errorf("%w: %s 声明类型为 %s，非 int", apiv1.ErrConfigInvalidValue, key, cfg.ValueType)
	}
	n, err := strconv.Atoi(cfg.ConfigValue)
	if err != nil {
		return 0, fmt.Errorf("%w: %w", apiv1.ErrConfigInvalidValue, err)
	}
	return n, nil
}

func (s *Service) GroupedConfigs(ctx context.Context) (*apiv1.GetConfigsResponseData, error) {
	list, err := s.repo.ListAll(ctx)
	if err != nil {
		s.logger.WithContext(ctx).Error("获取配置列表失败", zap.Error(err))
		return nil, err
	}
	// list 已按 config_group 排序，顺序追加即可保持分组内有序且组间稳定。
	data := &apiv1.GetConfigsResponseData{Groups: make([]apiv1.ConfigGroupItem, 0)}
	groupIdx := map[string]int{}
	for _, c := range list {
		idx, ok := groupIdx[c.ConfigGroup]
		if !ok {
			idx = len(data.Groups)
			groupIdx[c.ConfigGroup] = idx
			data.Groups = append(data.Groups, apiv1.ConfigGroupItem{
				Group: c.ConfigGroup,
				Items: make([]apiv1.SysConfigItem, 0),
			})
		}
		data.Groups[idx].Items = append(data.Groups[idx].Items, toItem(c))
	}
	return data, nil
}

func (s *Service) PublicConfigs(ctx context.Context) (*apiv1.GetPublicConfigsResponseData, error) {
	list, err := s.repo.ListPublic(ctx)
	if err != nil {
		s.logger.WithContext(ctx).Error("获取公开配置失败", zap.Error(err))
		return nil, err
	}
	data := &apiv1.GetPublicConfigsResponseData{List: make([]apiv1.PublicConfigItem, 0, len(list))}
	for _, c := range list {
		data.List = append(data.List, apiv1.PublicConfigItem{
			ConfigKey:   c.ConfigKey,
			ConfigValue: c.ConfigValue,
			ValueType:   c.ValueType,
		})
	}
	return data, nil
}

func (s *Service) Create(ctx context.Context, req *apiv1.ConfigCreateRequest) error {
	// configKey 是稳定契约，首尾空白会让后续按 key 读取难以排查，统一收敛。
	key := strings.TrimSpace(req.ConfigKey)
	group := strings.TrimSpace(req.ConfigGroup)
	title := strings.TrimSpace(req.Title)
	if key == "" || group == "" || title == "" {
		return fmt.Errorf("%w: 配置键、分组、名称不能为空白", apiv1.ErrBadRequest)
	}
	value, err := normalizeValue(req.ValueType, req.ConfigValue)
	if err != nil {
		return err
	}
	// 自定义配置一律 IsSystem=false：内置标记只由 seed 写入，不开放给接口。
	return s.repo.Create(ctx, &model.SysConfig{
		ConfigKey:   key,
		ConfigValue: value,
		ValueType:   req.ValueType,
		ConfigGroup: group,
		Title:       title,
		Remark:      req.Remark,
		IsPublic:    req.IsPublic,
		Weight:      req.Weight,
	})
}

// BatchUpdate 全校验通过才落库：先取快照逐项校验类型，再交 Repo 单事务写入。
func (s *Service) BatchUpdate(ctx context.Context, req *apiv1.BatchUpdateConfigRequest) error {
	kv := make(map[string]string, len(req.List))
	for _, item := range req.List {
		if _, dup := kv[item.ConfigKey]; dup {
			return fmt.Errorf("%w: 配置键重复 %s", apiv1.ErrBadRequest, item.ConfigKey)
		}
		kv[item.ConfigKey] = item.ConfigValue
	}

	keys := make([]string, 0, len(kv))
	for k := range kv {
		keys = append(keys, k)
	}
	snapshot, err := s.repo.FindByKeys(ctx, keys)
	if err != nil {
		s.logger.WithContext(ctx).Error("批量更新取配置快照失败", zap.Error(err))
		return err
	}
	if len(snapshot) != len(kv) {
		return apiv1.ErrConfigKeyNotFound
	}
	for _, c := range snapshot {
		value, err := normalizeValue(c.ValueType, kv[c.ConfigKey])
		if err != nil {
			return fmt.Errorf("%s: %w", c.ConfigKey, err)
		}
		kv[c.ConfigKey] = value
	}
	return s.repo.BatchUpdateValues(ctx, kv)
}

func (s *Service) Delete(ctx context.Context, id uint) error {
	cfg, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if cfg.IsSystem {
		return apiv1.ErrConfigSystemProtected
	}
	return s.repo.DeleteByID(ctx, id)
}

// normalizeValue 校验值能按声明类型解析，并返回规范化后的存储形式。
//
// 规范化保证库里只存唯一写法（bool 收敛为 true/false，int 去除前导零与正号），
// 避免 strconv.ParseBool 接受的 "1"/"t"/"TRUE" 等变体与前端的严格比较不一致。
func normalizeValue(valueType, value string) (string, error) {
	switch valueType {
	case model.ConfigTypeString:
		return value, nil
	case model.ConfigTypeInt:
		n, err := strconv.Atoi(value)
		if err != nil {
			return "", fmt.Errorf("%w: %w", apiv1.ErrConfigInvalidValue, err)
		}
		return strconv.Itoa(n), nil
	case model.ConfigTypeBool:
		b, err := strconv.ParseBool(value)
		if err != nil {
			return "", fmt.Errorf("%w: %w", apiv1.ErrConfigInvalidValue, err)
		}
		return strconv.FormatBool(b), nil
	case model.ConfigTypeJSON:
		if !json.Valid([]byte(value)) {
			return "", fmt.Errorf("%w: 非法 JSON", apiv1.ErrConfigInvalidValue)
		}
		return value, nil
	default:
		return "", fmt.Errorf("%w: 未知值类型 %s", apiv1.ErrConfigInvalidValue, valueType)
	}
}

func toItem(c model.SysConfig) apiv1.SysConfigItem {
	return apiv1.SysConfigItem{
		ID:          c.ID,
		ConfigKey:   c.ConfigKey,
		ConfigValue: c.ConfigValue,
		ValueType:   c.ValueType,
		ConfigGroup: c.ConfigGroup,
		Title:       c.Title,
		Remark:      c.Remark,
		IsPublic:    c.IsPublic,
		IsSystem:    c.IsSystem,
		Weight:      c.Weight,
		UpdatedAt:   c.UpdatedAt.Format("2006-01-02 15:04:05"),
	}
}
