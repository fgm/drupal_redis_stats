package redis_test

import (
	"testing"

	"github.com/fgm/drupal_redis_stats/redis"
)

func TestDSNValue(t *testing.T) {
	var v redis.DSNValue
	v.Set(v.Type())
	if v.String() != "string" {
		t.Errorf("Expected string got %s", v.String())
	}
}

func TestJSONValue(t *testing.T) {
	var v redis.JSONValue
	v.Set(v.Type())
	if v.String() != "false" { // "bool" is not trueish
		t.Errorf("Expected false got %s", v.String())
	}
}

func TestPassValue(t *testing.T) {
	var v redis.PassValue
	v.Set(v.Type())
	if v.String() != "string" {
		t.Errorf("Expected string got %s", v.String())
	}
}

func TestQuietValue(t *testing.T) {
	var v redis.QuietValue
	v.Set(v.Type())
	if v.String() != "false" { // "bool" is not trueish
		t.Errorf("Expected string got %s", v.String())
	}
}

func TestUserValue(t *testing.T) {
	var v redis.UserValue
	v.Set(v.Type())
	if v.String() != "string" {
		t.Errorf("Expected string got %s", v.String())
	}
}
