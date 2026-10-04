package engine

import (
	"context"
	"fmt"

	"github.com/deepnoodle-ai/risor/v2"
)

type RisorEngine struct {
	builtins map[string]any
}

func NewRisorEngine() *RisorEngine {
	builtins := risor.Builtins()
	delete(builtins, "rand")
	return &RisorEngine{builtins: builtins}
}

func (e *RisorEngine) Execute(
	ctx context.Context,
	script string,
	env map[string]any,
) (any, error) {
	opts := []risor.Option{risor.WithEnv(e.builtins), risor.WithEnv(env)}
	code, err := risor.Compile(ctx, script, opts...)
	if err != nil {
		return nil, fmt.Errorf("compiling risor: %w", err)
	}

	output, err := risor.Run(ctx, code, opts...)
	if err != nil {
		return nil, fmt.Errorf("running risor: %w", err)
	}
	return output, nil
}
