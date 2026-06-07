package meta

import (
	"crypto/sha1"
	"fmt"
	"regexp"
	"strings"
)

// Kubernetes の名前・ラベル値の最大長 (RFC 1123)。
const maxNameLen = 63

// プロジェクトの Namespace 名に付ける固定プレフィックス。
const namespacePrefix = "maxicloud-"

// 先頭・末尾の英数字以外を取り除く。識別子は英数字で開始・終了する必要がある。
var trimEdge = regexp.MustCompile(`^[^A-Za-z0-9]+|[^A-Za-z0-9]+$`)

// 連続するハイフンを1つにまとめる。
var collapseDash = regexp.MustCompile(`-+`)

// charset は変換先の識別子が許可する文字種ルール。
// 任意の入力文字列を、この charset で有効かつ長さ制限内の識別子へ変換する。
type charset struct {
	fallback  string         // 正規化結果が空になったときの既定 base
	lowercase bool           // 小文字のみ許可するか (DNS ラベル)
	invalid   *regexp.Regexp // 許可外文字の連続。ハイフンへ置換する
}

var (
	// DNS-1123 ラベル: 小文字英数字とハイフンのみ (Namespace 名)。
	dnsLabel = charset{
		fallback:  "project",
		lowercase: true,
		invalid:   regexp.MustCompile(`[^a-z0-9-]+`),
	}
	// Kubernetes ラベル値: 英数字とドット・アンダースコア・ハイフン。
	labelValue = charset{
		fallback: "branch",
		invalid:  regexp.MustCompile(`[^A-Za-z0-9._-]+`),
	}
)

// encode は raw を charset で有効な識別子へ変換し、prefix と
// stable key 由来のハッシュ接尾辞を付けて maxNameLen に収める。
// 同じ base に正規化される入力も、ハッシュにより衝突しない。
func (c charset) encode(raw, stableKey, prefix string) string {
	suffix := hashSuffix(stableKey)
	base := c.fit(c.normalize(raw), maxNameLen-len(prefix)-len(suffix))
	return prefix + base + suffix
}

// normalize は raw を charset で有効な base 文字列へ変換する (空になりうる)。
func (c charset) normalize(raw string) string {
	s := strings.TrimSpace(raw)
	if c.lowercase {
		s = strings.ToLower(s)
	}
	s = c.invalid.ReplaceAllString(s, "-")
	s = collapseDash.ReplaceAllString(s, "-")
	return trimEdge.ReplaceAllString(s, "")
}

// fit は base を budget 文字以内に収める。切り詰めで末尾に無効文字が残ることがあるため再トリムし、空になれば fallback を返す。
func (c charset) fit(base string, budget int) string {
	budget = max(budget, 1)
	if len(base) > budget {
		base = trimEdge.ReplaceAllString(base[:budget], "")
	}
	if base == "" {
		return c.fallback
	}
	return base
}

// hashSuffix は stableKey から "-xxxxxxxx" (16進8桁) の接尾辞を作る。
func hashSuffix(stableKey string) string {
	sum := sha1.Sum([]byte(strings.TrimSpace(stableKey)))
	return fmt.Sprintf("-%x", sum[:4])
}

// ProjectNamespace はプロジェクト名から一意な Namespace 名を作る。
// 同名プロジェクトでも stableKey (プロジェクト ID 等) で区別される。
func ProjectNamespace(name, stableKey string) string {
	return dnsLabel.encode(name, stableKey, namespacePrefix)
}

// PreviewProjectNamespace はプロジェクト ID から決定的な preview Namespace 名を作る。
func PreviewProjectNamespace(originalProjectID string) string {
	// TODO: 今後もし Namespace 名で どのプロジェクトの Preview かを識別したくなったら name 引数も受け取るようにする
	return dnsLabel.encode("preview", originalProjectID, namespacePrefix)
}

// branchLabelValue はブランチ名を Kubernetes ラベル値へ変換する。
func branchLabelValue(branch string) string {
	return labelValue.encode(branch, branch, "")
}

// clampLabelValue は正規化済みのラベル値を最大長に収める。
func clampLabelValue(raw string) string {
	if len(raw) <= maxNameLen {
		return raw
	}
	return raw[:maxNameLen]
}
