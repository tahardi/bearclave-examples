package networking

import (
	"bytes"
	"crypto/sha256"
	"errors"
)

var ErrIaCDigestMismatch = errors.New("iac digest mismatch")

func IaCDigest(script string, plan []byte) []byte {
	scriptHash := sha256.Sum256([]byte(script))
	planHash := sha256.Sum256(plan)
	return append(scriptHash[:], planHash[:]...)
}

func VerifyIaCPlan(userData []byte, script string, plan []byte) error {
	if !bytes.Equal(userData, IaCDigest(script, plan)) {
		return ErrIaCDigestMismatch
	}
	return nil
}
