package engine_test

import (
	"context"
	"fmt"
	"testing"

	"bearclave-examples/internal/engine"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExprEngine_Execute(t *testing.T) {
	sprintf := func(params ...any) (any, error) {
		if len(params) < 2 {
			return nil, fmt.Errorf("sprintf requires at least two arguments")
		}

		format, ok := params[0].(string)
		if !ok {
			return nil, fmt.Errorf("first argument must be a string")
		}
		return fmt.Sprintf(format, params[1:]...), nil
	}
	whitelist := map[string]engine.ExprEngineFn{
		"sprintf": sprintf,
	}

	t.Run("happy path", func(t *testing.T) {
		// given
		expression := `sprintf(greet, names[0])`
		env := map[string]any{
			"greet": "Hello, %v!",
			"names": []string{"world", "you"},
		}
		want := "Hello, world!"

		exprEngine, err := engine.NewExprEngineWithWhitelist(whitelist)
		require.NoError(t, err)

		// when
		got, err := exprEngine.Execute(context.Background(), expression, env)

		// then
		require.NoError(t, err)

		gotString, ok := got.(string)
		require.True(t, ok)
		require.Equal(t, want, gotString)
	})

	t.Run("error - compiling", func(t *testing.T) {
		// given
		expression := `sprintf(greet, names[0])`
		env := map[string]any{
			"greet": "Hello, %v!",
			"names": []string{"world", "you"},
		}

		exprEngine, err := engine.NewExprEngine()
		require.NoError(t, err)

		// when
		_, err = exprEngine.Execute(context.Background(), expression, env)

		// then
		assert.ErrorContains(t, err, "compile error")
	})

	t.Run("error - running", func(t *testing.T) {
		// given
		expression := `sprintf(greet)`
		env := map[string]any{
			"greet": "Hello, %v!",
			"names": []string{"world", "you"},
		}

		exprEngine, err := engine.NewExprEngineWithWhitelist(whitelist)
		require.NoError(t, err)

		// when
		_, err = exprEngine.Execute(context.Background(), expression, env)

		// then
		assert.ErrorContains(t, err, "running expr")
	})
}
