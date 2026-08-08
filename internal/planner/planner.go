package planner

import (
	"context"

	"github.com/audioprompters/pigments-web-mvp/internal/arturia"
)

// Request is the model-facing sound-design planning request. The model returns
// a schema-locked plan; the deterministic Arturia package applies it.
type Request struct {
	Mode          string
	Instruction   string
	PresetContext string
	Model         string
	Author        string
	Category      string
	Tags          []string
}

// Planner converts natural-language sound intent into safe parameter changes.
type Planner interface {
	Plan(context.Context, Request) (*arturia.PresetPlan, error)
	Status(context.Context) Status
}

// Status is exposed to the website health endpoint without leaking secrets.
type Status struct {
	Provider string `json:"provider"`
	Ready    bool   `json:"ready"`
	Model    string `json:"model,omitempty"`
	Endpoint string `json:"endpoint,omitempty"`
	Mock     bool   `json:"mock"`
	Message  string `json:"message,omitempty"`
}
