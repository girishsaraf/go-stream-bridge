package connectors_test

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	_ "gostreambridge/connectors"
	"gostreambridge/pkg/bridge"
	"gostreambridge/pkg/bridge/registry"
)

func TestFirstPartyFactoriesAreRegistered(t *testing.T) {
	if got, want := registry.Sources(), []string{"kafka", "rabbitmq"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("sources = %v, want %v", got, want)
	}
	if got, want := registry.Sinks(), []string{"elasticsearch", "kafka", "mysql", "sqlserver"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("sinks = %v, want %v", got, want)
	}
}

func TestFirstPartyFactoriesValidateConfiguration(t *testing.T) {
	for _, connector := range []struct {
		name         string
		registryName string
		new          func(context.Context, string, json.RawMessage) error
	}{
		{
			name:         "kafka source",
			registryName: "kafka",
			new: func(ctx context.Context, name string, raw json.RawMessage) error {
				_, err := registry.NewSource(ctx, name, raw, bridge.Dependencies{})
				return err
			},
		},
		{
			name:         "rabbitmq source",
			registryName: "rabbitmq",
			new: func(ctx context.Context, name string, raw json.RawMessage) error {
				_, err := registry.NewSource(ctx, name, raw, bridge.Dependencies{})
				return err
			},
		},
		{
			name:         "kafka sink",
			registryName: "kafka",
			new: func(ctx context.Context, name string, raw json.RawMessage) error {
				_, err := registry.NewSink(ctx, name, raw, bridge.Dependencies{})
				return err
			},
		},
		{
			name:         "mysql sink",
			registryName: "mysql",
			new: func(ctx context.Context, name string, raw json.RawMessage) error {
				_, err := registry.NewSink(ctx, name, raw, bridge.Dependencies{})
				return err
			},
		},
		{
			name:         "sqlserver sink",
			registryName: "sqlserver",
			new: func(ctx context.Context, name string, raw json.RawMessage) error {
				_, err := registry.NewSink(ctx, name, raw, bridge.Dependencies{})
				return err
			},
		},
		{
			name:         "elasticsearch sink",
			registryName: "elasticsearch",
			new: func(ctx context.Context, name string, raw json.RawMessage) error {
				_, err := registry.NewSink(ctx, name, raw, bridge.Dependencies{})
				return err
			},
		},
	} {
		t.Run(connector.name, func(t *testing.T) {
			if err := connector.new(context.Background(), connector.registryName, json.RawMessage(`{"configDir":""}`)); err == nil {
				t.Fatal("expected invalid connector configuration to fail")
			}
		})
	}
}
