package config

import (
    "math"
    "strings"
    "testing"
)

func TestRejectNonfiniteMaxRuntimeSeconds(t *testing.T) {
    payload := []byte(`{"schema":"WORKBRIDGE_CONFIG_V1","read_roots":[],"write_roots":[],"limits":{},"process":{"enabled":false},"http":{}}`)
    cfg, err := Parse(payload)
    if err != nil { t.Fatal(err) }
    for _, v := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
        cfg.Process.MaxRuntimeSeconds = v
        if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "finite") {
            t.Fatalf("nonfinite runtime admitted: %v, error: %v", v, err)
        }
    }
}
