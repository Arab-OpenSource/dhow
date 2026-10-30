package gitref

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseAcceptsTheFourForms(t *testing.T) {
	tests := []struct {
		full   string
		kind   Kind
		name   string
		number int
	}{
		{"refs/heads/main", KindBranch, "main", 0},
		{"refs/heads/feature/login-page", KindBranch, "feature/login-page", 0},
		{"refs/heads/release-1.2", KindBranch, "release-1.2", 0},
		{"refs/heads/إصلاح", KindBranch, "إصلاح", 0},
		{"refs/tags/v1.0.0", KindTag, "v1.0.0", 0},
		{"refs/pull/42/head", KindPullRequest, "42", 42},
		{"refs/merge-requests/7/head", KindMergeRequest, "7", 7},
	}
	for _, tt := range tests {
		t.Run(tt.full, func(t *testing.T) {
			ref, err := Parse(tt.full)
			require.NoError(t, err)
			assert.Equal(t, Ref{Full: tt.full, Kind: tt.kind, Name: tt.name, Number: tt.number}, ref)
		})
	}
}

func TestParseRefusesDangerousRefs(t *testing.T) {
	tests := []struct {
		name string
		ref  string
		want error
	}{
		{"empty", "", ErrEmpty},
		{"too long", "refs/heads/" + strings.Repeat("a", 250), ErrTooLong},
		{"a bare branch name", "main", ErrForm},
		{"an option as a ref", "-oProxyCommand=touch /tmp/x", ErrCharacter},
		{"an option without spaces", "--upload-pack=touch", ErrForm},
		{"an option as a branch name", "refs/heads/-oProxyCommand", ErrName},
		{"an upload-pack option as a branch", "refs/heads/--upload-pack=x", ErrName},
		{"two dots", "refs/heads/a..b", ErrName},
		{"a parent path", "refs/heads/../../config", ErrName},
		{"reflog syntax", "refs/heads/main@{1}", ErrName},
		{"only at", "refs/heads/@", ErrName},
		{"a lock file", "refs/heads/main.lock", ErrName},
		{"a hidden component", "refs/heads/.hidden", ErrName},
		{"a trailing dot", "refs/heads/main.", ErrName},
		{"a trailing slash", "refs/heads/main/", ErrName},
		{"a double slash", "refs/heads/a//b", ErrName},
		{"a newline", "refs/heads/main\nrefs/heads/evil", ErrCharacter},
		{"a carriage return", "refs/heads/main\r", ErrCharacter},
		{"a nul byte", "refs/heads/ma\x00in", ErrCharacter},
		{"an escape", "refs/heads/\x1b[31m", ErrCharacter},
		{"a delete", "refs/heads/a\x7f", ErrCharacter},
		{"a space", "refs/heads/my branch", ErrCharacter},
		{"a tilde", "refs/heads/main~1", ErrCharacter},
		{"a caret", "refs/heads/main^", ErrCharacter},
		{"a colon", "refs/heads/a:b", ErrCharacter},
		{"a question mark", "refs/heads/a?", ErrCharacter},
		{"a star", "refs/heads/*", ErrCharacter},
		{"a bracket", "refs/heads/[a]", ErrCharacter},
		{"a backslash", "refs/heads/a\\b", ErrCharacter},
		{"a right-to-left override", "refs/heads/main‮exe.", ErrCharacter},
		{"a zero width space", "refs/heads/ma​in", ErrCharacter},
		{"a no-break space", "refs/heads/ma in", ErrCharacter},
		{"invalid utf-8", "refs/heads/\xff", ErrCharacter},
		{"ext transport", "ext::sh -c touch% /tmp/pwned", ErrCharacter},
		{"a file url", "file:///etc/passwd", ErrCharacter},
		{"another namespace", "refs/remotes/origin/main", ErrForm},
		{"a pull request without head", "refs/pull/1", ErrForm},
		{"a pull request merge ref", "refs/pull/1/merge", ErrForm},
		{"a pull request number with a leading zero", "refs/pull/01/head", ErrForm},
		{"a pull request number zero", "refs/pull/0/head", ErrForm},
		{"a pull request number with letters", "refs/pull/1a/head", ErrForm},
		{"a huge pull request number", "refs/pull/12345678901/head", ErrForm},
		{"an empty branch", "refs/heads/", ErrEmpty},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse(tt.ref)
			require.ErrorIs(t, err, tt.want)
		})
	}
}

func TestBranchAndTagBuildFullRefs(t *testing.T) {
	ref, err := Branch("main")
	require.NoError(t, err)
	assert.Equal(t, "refs/heads/main", ref.Full)
	ref, err = Tag("v2")
	require.NoError(t, err)
	assert.Equal(t, KindTag, ref.Kind)
	_, err = Branch("-x")
	require.ErrorIs(t, err, ErrName)
}

func TestSha(t *testing.T) {
	good := []string{strings.Repeat("a", 40), strings.Repeat("0", 40), strings.Repeat("f", 64), "0123456789abcdef0123456789abcdef01234567"}
	for _, s := range good {
		assert.True(t, ValidSha(s), s)
		require.NoError(t, CheckSha(s))
	}
	bad := []string{"", strings.Repeat("a", 39), strings.Repeat("a", 41), strings.Repeat("A", 40), strings.Repeat("g", 40), "-" + strings.Repeat("a", 39), strings.Repeat("a", 63)}
	for _, s := range bad {
		assert.False(t, ValidSha(s), s)
		require.ErrorIs(t, CheckSha(s), ErrSha)
	}
	assert.True(t, IsZeroSha(strings.Repeat("0", 40)))
	assert.True(t, IsZeroSha(strings.Repeat("0", 64)))
	assert.False(t, IsZeroSha(strings.Repeat("0", 39)+"1"))
	assert.False(t, IsZeroSha("0"))
}

func TestKindNames(t *testing.T) {
	assert.Equal(t, "branch", KindBranch.String())
	assert.Equal(t, "tag", KindTag.String())
	assert.Equal(t, "pull_request", KindPullRequest.String())
	assert.Equal(t, "merge_request", KindMergeRequest.String())
	assert.Empty(t, Kind(0).String())
}

func FuzzRef(f *testing.F) {
	for _, seed := range []string{
		"refs/heads/main", "refs/tags/v1", "refs/pull/1/head", "refs/merge-requests/2/head", "refs/heads/a..b",
		"refs/heads/-x", "refs/heads/a@{1}", "--upload-pack=x", "ext::sh", "refs/heads/‮", "refs/heads/a/.b",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, full string) {
		ref, err := Parse(full)
		if err != nil {
			return
		}
		if ref.Full != full || len(full) > MaxRefBytes {
			t.Fatalf("accepted %q as %+v", full, ref)
		}
		if strings.HasPrefix(full, "-") || strings.Contains(full, "..") || strings.Contains(full, "@{") {
			t.Fatalf("accepted a dangerous ref %q", full)
		}
		for _, r := range full {
			if r <= ' ' || r == 0x7f || invisible(r) {
				t.Fatalf("accepted a control or invisible character in %q", full)
			}
		}
		for _, part := range strings.Split(ref.Name, "/") {
			if strings.HasPrefix(part, "-") || strings.HasPrefix(part, ".") {
				t.Fatalf("accepted a component %q in %q", part, full)
			}
		}
		if (ref.Kind == KindPullRequest || ref.Kind == KindMergeRequest) && ref.Number < 1 {
			t.Fatalf("accepted %q without a number", full)
		}
	})
}
