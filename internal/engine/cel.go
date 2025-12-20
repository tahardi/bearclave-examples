package engine

import (
	"context"
	"fmt"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
)

// maxVariadicArgs defines the maximum number of arguments supported for
// whitelisted functions. Unlike Expr, CEL does not support variadic functions,
// so we must define overloads for each number of arguments.
const maxVariadicArgs = 8

type CELEngineFn func(params ...any) (any, error)

type CELEngine struct {
	baseEnv *cel.Env
}

func NewCELEngine() (*CELEngine, error) {
	return NewCELEngineWithWhitelist(map[string]CELEngineFn{})
}

func NewCELEngineWithWhitelist(
	whitelist map[string]CELEngineFn,
) (*CELEngine, error) {
	opts := MakeWhitelistedFnOpts(whitelist)
	baseEnv, err := cel.NewEnv(opts...)
	if err != nil {
		return nil, fmt.Errorf("creating base CEL env: %w", err)
	}
	return &CELEngine{baseEnv: baseEnv}, nil
}

func (e *CELEngine) Execute(
	ctx context.Context,
	expression string,
	env map[string]any,
) (any, error) {
	opts := []cel.EnvOption{}
	for k := range env {
		opts = append(opts, cel.Variable(k, cel.DynType))
	}

	// Extend the pre-configured base environment with request-specific variables
	celEnv, err := e.baseEnv.Extend(opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to extend CEL env: %w", err)
	}

	ast, iss := celEnv.Compile(expression)
	if iss.Err() != nil {
		return nil, fmt.Errorf("compile error: %w", iss.Err())
	}

	program, err := celEnv.Program(ast)
	if err != nil {
		return nil, fmt.Errorf("program construction error: %w", err)
	}

	resultChan := make(chan any, 1)
	errChan := make(chan error, 1)
	go func() {
		out, _, err := program.ContextEval(ctx, env)
		if err != nil {
			errChan <- err
			return
		}
		resultChan <- out.Value()
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case err := <-errChan:
		return nil, fmt.Errorf("running cel: %w", err)
	case res := <-resultChan:
		return res, nil
	}
}

func MakeWhitelistedFnOpts(whitelist map[string]CELEngineFn) []cel.EnvOption {
	opts := []cel.EnvOption{}
	for name, fn := range whitelist {
		// Create a copy of fn because it used in a closure in MakeCELFnBinding
		localFn := fn
		overloads := []cel.FunctionOpt{}
		for i := 0; i <= maxVariadicArgs; i++ {
			overloads = append(overloads, MakeCELOverloadFunction(localFn, name, i))
		}
		opts = append(opts, cel.Function(name, overloads...))
	}
	return opts
}

func MakeCELOverloadFunction(
	fn CELEngineFn,
	name string,
	numArgs int,
) cel.FunctionOpt {
	argTypes := make([]*cel.Type, numArgs)
	for j := range argTypes {
		argTypes[j] = cel.DynType
	}

	overload := cel.Overload(
		fmt.Sprintf("%s_overload_%d", name, numArgs),
		argTypes,
		cel.DynType,
		cel.FunctionBinding(MakeCELFnBinding(fn)),
	)
	return overload
}

func MakeCELFnBinding(fn CELEngineFn) func(args ...ref.Val) ref.Val {
	return func(args ...ref.Val) ref.Val {
		params := make([]any, len(args))
		for i, arg := range args {
			params[i] = arg.Value()
		}
		res, err := fn(params...)
		if err != nil {
			return types.NewErr("%s", err.Error())
		}
		return types.DefaultTypeAdapter.NativeToValue(res)
	}
}