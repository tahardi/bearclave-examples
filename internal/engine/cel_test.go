package engine_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"bearclave-examples/internal/engine"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCELEngine_Execute(t *testing.T) {
	sprintf := func(params ...any) (any, error) {
		if len(params) < 2 {
			// nolint:err113
			return nil, errors.New("sprintf requires at least two arguments")
		}

		format, ok := params[0].(string)
		if !ok {
			// nolint:err113
			return nil, errors.New("first argument must be a string")
		}
		return fmt.Sprintf(format, params[1:]...), nil
	}
	whitelist := map[string]engine.CELEngineFn{
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

		celEngine, err := engine.NewCELEngineWithWhitelist(whitelist)
		require.NoError(t, err)

		// when
		got, err := celEngine.Execute(context.Background(), expression, env)

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

		celEngine, err := engine.NewCELEngine()
		require.NoError(t, err)

		// when
		_, err = celEngine.Execute(context.Background(), expression, env)

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

		celEngine, err := engine.NewCELEngineWithWhitelist(whitelist)
		require.NoError(t, err)

		// when
		_, err = celEngine.Execute(context.Background(), expression, env)

		// then
		assert.ErrorContains(t, err, "running cel")
	})
}
