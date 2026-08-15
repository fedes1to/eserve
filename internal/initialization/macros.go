package initialization

import (
	"fmt"
	"log"
)

type InitStep struct {
	Name     string
	Function func() error
}

func MustInit(steps []InitStep) {
	for _, step := range steps {
		if err := step.Function(); err != nil {
			log.Fatalf("failed %s: %v", step.Name, err)
		}
	}
}

func MustRegister(steps []InitStep) (error, int) {
	for _, step := range steps {
		if err := step.Function(); err != nil {
			return fmt.Errorf("failed %s: %w", step.Name, err), 1
		}
	}
	return nil, 0
}
