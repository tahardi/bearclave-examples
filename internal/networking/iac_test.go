package networking_test

import (
	"crypto/sha256"
	"testing"

	"github.com/tahardi/bearclave-examples/internal/networking"

	"github.com/stretchr/testify/assert"
)

func TestIaCDigest(t *testing.T) {
	// given
	script := `[{type: "aws_s3_bucket"}]`
	plan := []byte(`[{"type":"aws_s3_bucket"}]`)
	scriptHash := sha256.Sum256([]byte(script))
	planHash := sha256.Sum256(plan)
	want := append(scriptHash[:], planHash[:]...)

	// when
	got := networking.IaCDigest(script, plan)

	// then
	assert.Equal(t, want, got)
}

func TestVerifyIaCPlan(t *testing.T) {
	script := `[{type: "aws_s3_bucket"}]`
	plan := []byte(`[{"type":"aws_s3_bucket"}]`)
	userData := networking.IaCDigest(script, plan)

	tests := []struct {
		name     string
		userData []byte
		script   string
		plan     []byte
		wantErr  error
	}{
		{
			name:     "happy path",
			userData: userData,
			script:   script,
			plan:     plan,
		},
		{
			name:     "error - tampered plan",
			userData: userData,
			script:   script,
			plan:     []byte(`[{"type":"aws_iam_user"}]`),
			wantErr:  networking.ErrIaCDigestMismatch,
		},
		{
			name:     "error - tampered script",
			userData: userData,
			script:   `[{type: "aws_iam_user"}]`,
			plan:     plan,
			wantErr:  networking.ErrIaCDigestMismatch,
		},
		{
			name:     "error - missing user data",
			userData: nil,
			script:   script,
			plan:     plan,
			wantErr:  networking.ErrIaCDigestMismatch,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// when
			err := networking.VerifyIaCPlan(tc.userData, tc.script, tc.plan)

			// then
			assert.ErrorIs(t, err, tc.wantErr)
		})
	}
}
