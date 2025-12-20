package engine

import (
	"context"
	"fmt"

	"github.com/expr-lang/expr"
)

type ExprEngineFn func(params ...any) (any, error)

type ExprEngine struct {
	whitelist map[string]ExprEngineFn
}

func NewExprEngine() (*ExprEngine, error) {
	return NewExprEngineWithWhitelist(map[string]ExprEngineFn{})
}

func NewExprEngineWithWhitelist(
	whitelist map[string]ExprEngineFn,
) (*ExprEngine, error) {
	return &ExprEngine{whitelist: whitelist}, nil
}

func (e *ExprEngine) Execute(
	ctx context.Context,
	expression string,
	env map[string]any,
) (any, error) {
	whitelistedFns := []expr.Option{expr.Env(env)}
	for name, fn := range e.whitelist {
		whitelistedFns = append(whitelistedFns, expr.Function(name, fn))
	}

	program, err := expr.Compile(expression, whitelistedFns...)
	if err != nil {
		return nil, fmt.Errorf("compile error: %w", err)
	}

	resultChan := make(chan any, 1)
	errChan := make(chan error, 1)
	go func() {
		output, err := expr.Run(program, env)
		if err != nil {
			errChan <- err
			return
		}
		resultChan <- output
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case err := <-errChan:
		return nil, fmt.Errorf("running expr: %w", err)
	case res := <-resultChan:
		return res, nil
	}
}
