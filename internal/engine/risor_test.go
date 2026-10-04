package engine_test

import (
	"context"
	"testing"

	"github.com/tahardi/bearclave-examples/internal/engine"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRisorEngine_Execute(t *testing.T) {
	t.Run("happy path - plan is deterministic", func(t *testing.T) {
		// given
		script := `regions.map(r => ({type: "aws_s3_bucket", name: sprintf("%s-%s", prefix, r), region: r}))`
		env := map[string]any{
			"prefix":  "logs",
			"regions": []any{"us-east-1", "us-west-2"},
		}
		want := []any{
			map[string]any{"type": "aws_s3_bucket", "name": "logs-us-east-1", "region": "us-east-1"},
			map[string]any{"type": "aws_s3_bucket", "name": "logs-us-west-2", "region": "us-west-2"},
		}

		risorEngine := engine.NewRisorEngine()

		// when
		first, err := risorEngine.Execute(context.Background(), script, env)
		require.NoError(t, err)
		second, err := risorEngine.Execute(context.Background(), script, env)
		require.NoError(t, err)

		// then
		assert.Equal(t, want, first)
		assert.Equal(t, first, second)
	})

	t.Run("error - rand module is unavailable", func(t *testing.T) {
		// given
		script := `rand.int()`
		risorEngine := engine.NewRisorEngine()

		// when
		_, err := risorEngine.Execute(context.Background(), script, nil)

		// then
		assert.ErrorContains(t, err, "compiling risor")
	})

	t.Run("error - file access", func(t *testing.T) {
		// given
		script := `open("/etc/passwd").read()`
		risorEngine := engine.NewRisorEngine()

		// when
		_, err := risorEngine.Execute(context.Background(), script, nil)

		// then
		assert.ErrorContains(t, err, "compiling risor")
	})

	t.Run("error - network access", func(t *testing.T) {
		// given
		script := `fetch("http://example.com")`
		risorEngine := engine.NewRisorEngine()

		// when
		_, err := risorEngine.Execute(context.Background(), script, nil)

		// then
		assert.ErrorContains(t, err, "compiling risor")
	})

	t.Run("error - running", func(t *testing.T) {
		// given
		script := `throw error("bad plan")`
		risorEngine := engine.NewRisorEngine()

		// when
		_, err := risorEngine.Execute(context.Background(), script, nil)

		// then
		assert.ErrorContains(t, err, "running risor")
	})

	t.Run("error - context canceled", func(t *testing.T) {
		// given
		script := `range(1000000000).map(x => x)`
		risorEngine := engine.NewRisorEngine()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		// when
		_, err := risorEngine.Execute(ctx, script, nil)

		// then
		assert.ErrorIs(t, err, context.Canceled)
	})
}
