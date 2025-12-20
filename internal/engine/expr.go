package engine

import (
	"context"
	"fmt"

	"github.com/expr-lang/expr"
)

type ExprEngineFn func(params ...any) (any, error)

type ExprEngine struct {
	baseOptions []expr.Option
}

func NewExprEngine() (*ExprEngine, error) {
	return NewExprEngineWithWhitelist(map[string]ExprEngineFn{})
}

func NewExprEngineWithWhitelist(
	whitelist map[string]ExprEngineFn,
) (*ExprEngine, error) {
	baseOptions := make([]expr.Option, 0, len(whitelist))
	for name, fn := range whitelist {
		baseOptions = append(baseOptions, expr.Function(name, fn))
	}
	return &ExprEngine{baseOptions: baseOptions}, nil
}

func (e *ExprEngine) Execute(
	ctx context.Context,
	expression string,
	env map[string]any,
) (any, error) {
	program, err := expr.Compile(expression, append(e.baseOptions, expr.Env(env))...)
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
