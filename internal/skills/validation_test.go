package skills

import "testing"

// TestValidatePackageRejectsUnsafePaths verifies uploaded packages cannot smuggle unsafe paths.
// TestValidatePackageRejectsUnsafePaths 用于验证上传包不能携带不安全路径。
func TestValidatePackageRejectsUnsafePaths(t *testing.T) {
	t.Parallel()

	result := ValidatePackage(Package{
		Name: "unsafe",
		Files: map[string][]byte{
			"SKILL.md":          []byte("# unsafe"),
			"../scripts/run.sh": []byte("echo unsafe"),
		},
	}, NewPackageAdapter())
	if result.Valid {
		t.Fatalf("expected package validation to fail")
	}
	if len(result.Errors) == 0 {
		t.Fatalf("expected validation errors")
	}
}

// TestValidatePackageWarnsOnUnknownTopLevelPath verifies governance warnings survive valid uploads.
// TestValidatePackageWarnsOnUnknownTopLevelPath 用于验证治理告警会在有效上传中保留下来。
func TestValidatePackageWarnsOnUnknownTopLevelPath(t *testing.T) {
	t.Parallel()

	result := ValidatePackage(Package{
		Name: "warn",
		Files: map[string][]byte{
			"SKILL.md":           []byte("# warn"),
			"misc/note.txt":      []byte("note"),
			"references/doc.txt": []byte("doc"),
		},
	}, NewPackageAdapter())
	if !result.Valid {
		t.Fatalf("expected package validation to remain valid")
	}
	if len(result.Warnings) == 0 {
		t.Fatalf("expected validation warnings")
	}
}
