package openshift

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/integr8ly/cloud-resource-operator/internal/k8sutil"

	"github.com/integr8ly/cloud-resource-operator/pkg/resources"

	"github.com/integr8ly/cloud-resource-operator/pkg/providers"
	errorUtil "github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	DefaultConfigMapName = "cloud-resources-openshift-strategies"
	DefaultFinalizer     = "cloud-resources-operator.integreatly.org/finalizers"
	defaultReconcileTime = time.Second * 30

	NamespacePrefixOpenShift = "openshift-"
)

//DefaultConfigMapNamespace get default namespace
var DefaultConfigMapNamespace, _ = k8sutil.GetWatchNamespace()

type StrategyConfig struct {
	RawStrategy json.RawMessage `json:"strategy"`
}

//go:generate moq -out config_moq.go . ConfigManager
type ConfigManager interface {
	ReadStorageStrategy(ctx context.Context, rt providers.ResourceType, tier string) (*StrategyConfig, error)
}

type ConfigMapConfigManager struct {
	configMapName      string
	configMapNamespace string
	client             client.Client
}

var _ ConfigManager = (*ConfigMapConfigManager)(nil)

func NewConfigMapConfigManager(cm string, namespace string, client client.Client) *ConfigMapConfigManager {
	if cm == "" {
		cm = DefaultConfigMapName
	}
	if namespace == "" {
		namespace = DefaultConfigMapNamespace
	}
	return &ConfigMapConfigManager{
		configMapName:      cm,
		configMapNamespace: namespace,
		client:             client,
	}
}

func NewDefaultConfigManager(client client.Client) *ConfigMapConfigManager {
	return NewConfigMapConfigManager(DefaultConfigMapName, DefaultConfigMapNamespace, client)
}

func (m *ConfigMapConfigManager) ReadStorageStrategy(ctx context.Context, rt providers.ResourceType, tier string) (*StrategyConfig, error) {
	cm, err := resources.GetConfigMapOrDefault(ctx, m.client, types.NamespacedName{Name: m.configMapName, Namespace: m.configMapNamespace}, m.buildDefaultConfigMap())
	if err != nil {
		return nil, errorUtil.Wrapf(err, "failed to get openshift strategy config map %s in namespace %s", m.configMapName, m.configMapNamespace)
	}
	rawStrategyCfg := cm.Data[string(rt)]
	if rawStrategyCfg == "" {
		return nil, errorUtil.New(fmt.Sprintf("openshift strategy for resource type %s is not defined", rt))
	}

	var strategies map[string]*StrategyConfig
	if err = json.Unmarshal([]byte(rawStrategyCfg), &strategies); err != nil {
		return nil, errorUtil.Wrapf(err, "failed to unmarshal strategy mapping for resource type %s", rt)
	}
	if strategies[tier] == nil {
		return nil, errorUtil.New(fmt.Sprintf("no strategy found for deployment type %s and deployment tier %s", rt, tier))
	}
	return strategies[tier], nil
}

func (m *ConfigMapConfigManager) buildDefaultConfigMap() *v1.ConfigMap {
	return &v1.ConfigMap{
		ObjectMeta: controllerruntime.ObjectMeta{
			Name:      m.configMapName,
			Namespace: m.configMapNamespace,
		},
		Data: map[string]string{
			"postgres":    "{\"development\": { \"strategy\": {} }, \"production\": { \"strategy\": {} } }",
			"redis":       "{\"development\": { \"strategy\": {} }, \"production\": { \"strategy\": {} } }",
			"blobstorage": "{\"development\": { \"strategy\": {} }, \"production\": { \"strategy\": {} } }",
		},
	}
}
