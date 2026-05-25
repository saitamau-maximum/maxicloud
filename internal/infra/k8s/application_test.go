package k8s

import (
	"regexp"
	"strings"
	"testing"
)

func TestNormalizeBranchForLabel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		in           string
		expectPrefix string
	}{
		{
			name:         "許可文字はそのまま維持する",
			in:           "feature.foo_bar-1",
			expectPrefix: "feature.foo_bar-1-",
		},
		{
			name:         "スラッシュをハイフンに置換する",
			in:           "feature/login-flow",
			expectPrefix: "feature-login-flow-",
		},
		{
			name:         "連続する無効文字を1つのハイフンにまとめる",
			in:           "feature///login@@@flow",
			expectPrefix: "feature-login-flow-",
		},
		{
			name:         "先頭と末尾の英数字以外を除去する",
			in:           "--feature-login--",
			expectPrefix: "feature-login-",
		},
		{
			name:         "空文字の場合はフォールバックのベースを使う",
			in:           "",
			expectPrefix: "branch-",
		},
		{
			name:         "無効文字のみの場合はフォールバックのベースを使う",
			in:           "/////@@@@@-----",
			expectPrefix: "branch-",
		},
		{
			name:         "非ASCIIのみの場合はフォールバックのベースを使う",
			in:           "日本語ブランチ",
			expectPrefix: "branch-",
		},
		{
			name:         "前後の空白を除去する",
			in:           "  feature/new-ui  ",
			expectPrefix: "feature-new-ui-",
		},
	}

	hexSuffixPattern := regexp.MustCompile(`-[0-9a-f]{8}$`)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := normalizeBranchForLabel(tt.in)
			if !strings.HasPrefix(got, tt.expectPrefix) {
				t.Fatalf("normalizeBranchForLabel(%q) = %q, want prefix %q", tt.in, got, tt.expectPrefix)
			}
			if !hexSuffixPattern.MatchString(got) {
				t.Fatalf("normalizeBranchForLabel(%q) = %q, want suffix matching %q", tt.in, got, hexSuffixPattern.String())
			}
			if len(got) > 63 {
				t.Fatalf("normalizeBranchForLabel(%q) len = %d, want <= 63", tt.in, len(got))
			}
		})
	}
}

func TestNormalizeBranchForLabel_DifferentInputsDifferentOutputs(t *testing.T) {
	t.Parallel()

	a := normalizeBranchForLabel("feat/hogehoge-fugafuga")
	b := normalizeBranchForLabel("feat-hogehoge-fugafuga")
	if a == b {
		t.Fatalf("expected different outputs for potentially colliding branches, got %q", a)
	}
}
