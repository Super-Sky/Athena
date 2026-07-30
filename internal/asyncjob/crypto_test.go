package asyncjob

import (
	"bytes"
	"testing"
)

func TestCodecRoundTripAndAADIsolation(t *testing.T) {
	codec, err := NewCodec("0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatal(err)
	}
	encrypted, err := codec.Encrypt([]byte(`{"goal":"analyze"}`), "job-1/run-1")
	if err != nil {
		t.Fatal(err)
	}
	plain, err := codec.Decrypt(encrypted, "job-1/run-1")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(plain, []byte(`{"goal":"analyze"}`)) {
		t.Fatalf("plain=%s", plain)
	}
	if _, err := codec.Decrypt(encrypted, "job-2/run-1"); err == nil {
		t.Fatal("decrypt with mismatched AAD succeeded")
	}
}

func TestNewCodecRejectsShortKey(t *testing.T) {
	if _, err := NewCodec("short"); err == nil {
		t.Fatal("NewCodec() expected short-key error")
	}
}
