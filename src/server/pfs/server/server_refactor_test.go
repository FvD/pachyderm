package server

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"

	pclient "github.com/pachyderm/pachyderm/src/client"
	pfsclient "github.com/pachyderm/pachyderm/src/client/pfs"
	"github.com/pachyderm/pachyderm/src/client/pkg/require"
)

func TestBranchSimpleRF(t *testing.T) {
	t.Parallel()
	client, _ := getClientAndServer(t)

	repo := "test"
	require.NoError(t, client.CreateRepo(repo))

	commit, err := client.StartCommit(repo, "", "branchA")
	require.NoError(t, err)

	require.NoError(t, client.FinishCommit(repo, commit.ID))
	heads, err := client.ListBranch(repo)
	require.NoError(t, err)

	require.Equal(t, 1, len(heads))
	require.Equal(t, "branchA", heads[0].Branch)

}

func TestListBranchRF(t *testing.T) {
	t.Parallel()
	client, _ := getClientAndServer(t)

	repo := "test"
	require.NoError(t, client.CreateRepo(repo))

	commit1, err := client.StartCommit(repo, "", "branchA")
	require.NoError(t, err)

	require.NoError(t, client.FinishCommit(repo, commit1.ID))

	// Don't specify a branch because we already should have it from parent
	commit2, err := client.StartCommit(repo, commit1.ID, "")
	require.NoError(t, err)

	require.NoError(t, client.FinishCommit(repo, commit2.ID))

	// Specify branch, because branching off of commit1
	commit3, err := client.StartCommit(repo, commit1.ID, "branchB")
	require.NoError(t, err)

	require.NoError(t, client.FinishCommit(repo, commit3.ID))
	branches, err := client.ListBranch(repo)
	require.NoError(t, err)

	require.Equal(t, 2, len(branches))
	branchNames := []interface{}{
		branches[0].Branch,
		branches[1].Branch,
	}

	require.EqualOneOf(t, branchNames, "branchA")
	require.EqualOneOf(t, branchNames, "branchB")
}

func TestListCommitBasicRF(t *testing.T) {
	t.Parallel()
	client, _ := getClientAndServer(t)

	require.NoError(t, client.CreateRepo("test"))
	numCommits := 10
	var commitIDs []string
	for i := 0; i < numCommits; i++ {
		commit, err := client.StartCommit("test", "", "master")
		require.NoError(t, err)
		require.NoError(t, client.FinishCommit("test", commit.ID))
		commitIDs = append(commitIDs, commit.ID)
	}

	commitInfos, err := client.ListCommit(
		[]string{"test"},
		nil,
		pclient.CommitTypeNone,
		false,
		false,
		nil,
	)
	require.NoError(t, err)

	require.Equal(t, len(commitInfos), numCommits)
	for i, commitInfo := range commitInfos {
		require.Equal(t, commitIDs[len(commitIDs)-i-1], commitInfo.Commit.ID)
	}
}

func TestStartAndFinishCommitRF(t *testing.T) {
	t.Parallel()
	client, _ := getClientAndServer(t)

	repo := "test"
	require.NoError(t, client.CreateRepo(repo))

	commit, err := client.StartCommit(repo, "", "")
	require.NoError(t, err)

	require.NoError(t, client.FinishCommit(repo, commit.ID))
}

func TestInspectCommitBasicRF(t *testing.T) {
	t.Parallel()
	client, _ := getClientAndServer(t)

	repo := "test"
	require.NoError(t, client.CreateRepo(repo))

	started := time.Now()
	commit, err := client.StartCommit(repo, "", "")
	require.NoError(t, err)

	commitInfo, err := client.InspectCommit(repo, commit.ID)
	require.NoError(t, err)

	require.Equal(t, commit, commitInfo.Commit)
	require.Equal(t, pfsclient.CommitType_COMMIT_TYPE_WRITE, commitInfo.CommitType)
	require.Equal(t, 0, int(commitInfo.SizeBytes))
	require.True(t, started.Before(commitInfo.Started.GoTime()))
	require.Nil(t, commitInfo.Finished)

	require.NoError(t, client.FinishCommit(repo, commit.ID))
	finished := time.Now()

	commitInfo, err = client.InspectCommit(repo, commit.ID)
	require.NoError(t, err)

	require.Equal(t, commit.ID, commitInfo.Commit.ID)
	require.Equal(t, pfsclient.CommitType_COMMIT_TYPE_READ, commitInfo.CommitType)
	require.Equal(t, 0, int(commitInfo.SizeBytes))
	require.True(t, started.Before(commitInfo.Started.GoTime()))
	require.True(t, finished.After(commitInfo.Finished.GoTime()))
}

func TestStartCommitFromParentIDRF(t *testing.T) {
	t.Parallel()

	client, _ := getClientAndServer(t)

	repo := "test"
	require.NoError(t, client.CreateRepo(repo))

	commit, err := client.StartCommit(repo, "", "")
	require.NoError(t, err)

	require.NoError(t, client.FinishCommit(repo, commit.ID))

	branches, err := client.ListBranch(repo)
	require.NoError(t, err)

	require.Equal(t, 1, len(branches))

	// Should create commit off of parent on the same branch
	commit1, err := client.StartCommit(repo, commit.ID, "")
	require.NoError(t, err)

	require.NoError(t, client.FinishCommit(repo, commit1.ID))
	existingBranch := branches[0].Branch
	branches, err = client.ListBranch(repo)
	require.NoError(t, err)

	require.Equal(t, 1, len(branches))

	// Should create commit off of parent on a new branch by name
	commit2, err := client.StartCommit(repo, commit.ID, "foo")
	require.NoError(t, err)

	branches2, err := client.ListBranch(repo)
	require.NoError(t, err)

	uniqueBranches := make(map[string]bool)

	for _, thisBranch := range branches2 {
		uniqueBranches[thisBranch.Branch] = true
	}

	require.Equal(t, 2, len(uniqueBranches))
	delete(uniqueBranches, existingBranch)
	require.Equal(t, 1, len(uniqueBranches))

	require.NoError(t, client.FinishCommit(repo, commit2.ID))
}

func TestInspectRepoMostBasicRF(t *testing.T) {
	t.Parallel()
	client, _ := getClientAndServer(t)

	repo := "test"
	require.NoError(t, client.CreateRepo(repo))

	repoInfo, err := client.InspectRepo(repo)
	require.NoError(t, err)

	require.Equal(t, int(repoInfo.SizeBytes), 0)
}

func TestStartCommitLatestOnBranchRF(t *testing.T) {
	t.Parallel()
	client, _ := getClientAndServer(t)

	repo := "test"
	require.NoError(t, client.CreateRepo(repo))

	commit1, err := client.StartCommit(repo, "", "branchA")
	require.NoError(t, err)

	require.NoError(t, client.FinishCommit(repo, commit1.ID))

	commit2, err := client.StartCommit(repo, "", "branchA")
	require.NoError(t, err)

	require.NoError(t, client.FinishCommit(repo, commit2.ID))

	commit3, err := client.StartCommit(repo, "", "branchA")
	require.NoError(t, err)

	require.NoError(t, client.FinishCommit(repo, commit3.ID))

	branches, err := client.ListBranch(repo)
	require.Equal(t, 1, len(branches))
	require.Equal(t, commit3.ID, branches[0].Commit.ID)
}

func TestListBranchRedundantRF(t *testing.T) {
	t.Parallel()
	client, _ := getClientAndServer(t)

	repo := "test"
	require.NoError(t, client.CreateRepo(repo))

	commit1, err := client.StartCommit(repo, "", "branchA")
	require.NoError(t, err)

	require.NoError(t, client.FinishCommit(repo, commit1.ID))

	// Can't create branch if it exists
	_, err = client.StartCommit(repo, commit1.ID, "branchA")
	require.YesError(t, err)

	branches, err := client.ListBranch(repo)
	require.NoError(t, err)

	require.Equal(t, 1, len(branches))
	require.Equal(t, "branchA", branches[0].Branch)
}

func TestNEWAPIStartCommitFromBranchRF(t *testing.T) {
	t.Parallel()
	client, _ := getClientAndServer(t)

	repo := "test"
	require.NoError(t, client.CreateRepo(repo))

	_, err := client.StartCommit(repo, "", "master")
	require.NoError(t, err)

	commitInfo, err := client.InspectCommit(repo, "master/0")
	require.NoError(t, err)
	require.Equal(t, "master", commitInfo.Branch)
	require.Equal(t, "test", commitInfo.Commit.Repo.Name)

	require.NoError(t, client.FinishCommit(repo, "master/0"))
}

func TestNEWAPIStartCommitNewBranchRF(t *testing.T) {
	t.Parallel()
	client, _ := getClientAndServer(t)

	repo := "test"
	require.NoError(t, client.CreateRepo(repo))

	commit1, err := client.StartCommit(repo, "", "master")
	require.NoError(t, err)
	require.NoError(t, client.FinishCommit(repo, "master/0"))

	_, err = client.StartCommit(repo, commit1.ID, "foo")
	require.NoError(t, err)

	commitInfo, err := client.InspectCommit(repo, "foo/0")
	require.NoError(t, err)
	require.Equal(t, "foo", commitInfo.Branch)
	require.Equal(t, "test", commitInfo.Commit.Repo.Name)
}

func TestNEWAPIPutFileRF(t *testing.T) {
	t.Parallel()
	client, _ := getClientAndServer(t)
	repo := "test"
	require.NoError(t, client.CreateRepo(repo))
	commit1, err := client.StartCommit(repo, "", "master")
	require.NoError(t, err)
	_, err = client.PutFile(repo, commit1.ID, "file", strings.NewReader("foo\n"))
	require.NoError(t, err)
	_, err = client.PutFile(repo, commit1.ID, "file", strings.NewReader("bar\n"))
	require.NoError(t, err)
	_, err = client.PutFile(repo, "master/0", "file", strings.NewReader("buzz\n"))
	require.NoError(t, err)
	require.NoError(t, client.FinishCommit(repo, "master/0"))

	expected := "foo\nbar\nbuzz\n"
	buffer := &bytes.Buffer{}
	require.NoError(t, client.GetFile(repo, commit1.ID, "file", 0, 0, "", nil, buffer))
	require.Equal(t, expected, buffer.String())
	buffer.Reset()
	require.NoError(t, client.GetFile(repo, "master/0", "file", 0, 0, "", nil, buffer))
	require.Equal(t, expected, buffer.String())

	commit2, err := client.StartCommit(repo, "", "master")
	require.NoError(t, err)
	_, err = client.PutFile(repo, commit2.ID, "file", strings.NewReader("foo\n"))
	require.NoError(t, err)
	_, err = client.PutFile(repo, commit2.ID, "file", strings.NewReader("bar\n"))
	require.NoError(t, err)
	_, err = client.PutFile(repo, "master/1", "file", strings.NewReader("buzz\n"))
	require.NoError(t, err)
	require.NoError(t, client.FinishCommit(repo, "master/1"))

	expected = "foo\nbar\nbuzz\nfoo\nbar\nbuzz\n"
	buffer.Reset()
	require.NoError(t, client.GetFile(repo, commit2.ID, "file", 0, 0, "", nil, buffer))
	require.Equal(t, expected, buffer.String())
	buffer.Reset()
	require.NoError(t, client.GetFile(repo, "master/1", "file", 0, 0, "", nil, buffer))
	require.Equal(t, expected, buffer.String())

	_, err = client.StartCommit(repo, "master/1", "foo")
	require.NoError(t, err)
	_, err = client.PutFile(repo, "foo/0", "file", strings.NewReader("foo\nbar\nbuzz\n"))
	require.NoError(t, client.FinishCommit(repo, "foo/0"))

	expected = "foo\nbar\nbuzz\nfoo\nbar\nbuzz\nfoo\nbar\nbuzz\n"
	buffer.Reset()
	require.NoError(t, client.GetFile(repo, "foo/0", "file", 0, 0, "", nil, buffer))
	require.Equal(t, expected, buffer.String())
}

func TestNEWAPIDeleteFileRF(t *testing.T) {
	t.Parallel()
	client, _ := getClientAndServer(t)
	repo := "test"
	require.NoError(t, client.CreateRepo(repo))

	_, err := client.StartCommit(repo, "", "master")
	require.NoError(t, err)
	_, err = client.PutFile(repo, "master/0", "file", strings.NewReader("foo\n"))
	require.NoError(t, err)
	require.NoError(t, client.FinishCommit(repo, "master/0"))

	_, err = client.StartCommit(repo, "", "master")
	require.NoError(t, err)
	err = client.DeleteFile(repo, "master/1", "file", false, "")
	require.NoError(t, err)
	_, err = client.PutFile(repo, "master/1", "file", strings.NewReader("bar\n"))
	require.NoError(t, err)
	require.NoError(t, client.FinishCommit(repo, "master/1"))

	expected := "bar\n"
	var buffer bytes.Buffer
	require.NoError(t, client.GetFile(repo, "master/1", "file", 0, 0, "master/0", nil, &buffer))
	require.Equal(t, expected, buffer.String())

	_, err = client.StartCommit(repo, "", "master")
	require.NoError(t, err)
	_, err = client.PutFile(repo, "master/2", "file", strings.NewReader("buzz\n"))
	require.NoError(t, err)
	err = client.DeleteFile(repo, "master/2", "file", false, "")
	require.NoError(t, err)
	_, err = client.PutFile(repo, "master/2", "file", strings.NewReader("foo\n"))
	require.NoError(t, err)
	require.NoError(t, client.FinishCommit(repo, "master/2"))

	expected = "foo\n"
	buffer.Reset()
	require.NoError(t, client.GetFile(repo, "master/2", "file", 0, 0, "master/0", nil, &buffer))
	require.Equal(t, expected, buffer.String())
}

func TestNEWAPIInspectFileRF(t *testing.T) {
	t.Parallel()
	client, _ := getClientAndServer(t)
	repo := "test"
	require.NoError(t, client.CreateRepo(repo))

	fileContent1 := "foo\n"
	fileContent2 := "buzz\n"

	_, err := client.StartCommit(repo, "", "master")
	require.NoError(t, err)
	_, err = client.PutFile(repo, "master/0", "file", strings.NewReader(fileContent1))
	require.NoError(t, err)
	require.NoError(t, client.FinishCommit(repo, "master/0"))

	fileInfo, err := client.InspectFile(repo, "master/0", "file", "", nil)
	require.NoError(t, err)
	require.Equal(t, len(fileContent1), int(fileInfo.SizeBytes))
	require.Equal(t, "/file", fileInfo.File.Path)
	require.Equal(t, pfsclient.FileType_FILE_TYPE_REGULAR, fileInfo.FileType)

	_, err = client.StartCommit(repo, "", "master")
	require.NoError(t, err)
	_, err = client.PutFile(repo, "master/1", "file", strings.NewReader(fileContent1))
	require.NoError(t, err)
	require.NoError(t, client.FinishCommit(repo, "master/1"))

	fileInfo, err = client.InspectFile(repo, "master/1", "file", "", nil)
	require.NoError(t, err)
	require.Equal(t, len(fileContent1)*2, int(fileInfo.SizeBytes))
	require.Equal(t, "/file", fileInfo.File.Path)

	_, err = client.StartCommit(repo, "", "master")
	require.NoError(t, err)
	err = client.DeleteFile(repo, "master/2", "file", false, "")
	require.NoError(t, err)
	_, err = client.PutFile(repo, "master/2", "file", strings.NewReader(fileContent2))
	require.NoError(t, err)
	require.NoError(t, client.FinishCommit(repo, "master/2"))

	fileInfo, err = client.InspectFile(repo, "master/2", "file", "", nil)
	require.NoError(t, err)
	require.Equal(t, len(fileContent2), int(fileInfo.SizeBytes))
}

func TestNEWAPIInspectDirectoryRF(t *testing.T) {
	t.Parallel()
	client, _ := getClientAndServer(t)
	repo := "test"
	require.NoError(t, client.CreateRepo(repo))

	fileContent := "foo\n"

	_, err := client.StartCommit(repo, "", "master")
	require.NoError(t, err)
	_, err = client.PutFile(repo, "master/0", "dir/1", strings.NewReader(fileContent))
	require.NoError(t, err)
	_, err = client.PutFile(repo, "master/0", "dir/2", strings.NewReader(fileContent))
	require.NoError(t, err)
	require.NoError(t, client.FinishCommit(repo, "master/0"))

	fileInfo, err := client.InspectFile(repo, "master/0", "dir", "", nil)
	require.NoError(t, err)
	require.Equal(t, 2, len(fileInfo.Children))
	require.Equal(t, "/dir", fileInfo.File.Path)
	require.Equal(t, pfsclient.FileType_FILE_TYPE_DIR, fileInfo.FileType)

	_, err = client.StartCommit(repo, "", "master")
	require.NoError(t, err)
	_, err = client.PutFile(repo, "master/1", "dir/3", strings.NewReader(fileContent))
	require.NoError(t, err)
	require.NoError(t, client.FinishCommit(repo, "master/1"))

	fileInfo, err = client.InspectFile(repo, "master/1", "dir", "", nil)
	require.NoError(t, err)
	require.Equal(t, 3, len(fileInfo.Children))

	_, err = client.StartCommit(repo, "", "master")
	require.NoError(t, err)
	err = client.DeleteFile(repo, "master/2", "dir/2", false, "")
	require.NoError(t, err)
	require.NoError(t, client.FinishCommit(repo, "master/2"))

	fileInfo, err = client.InspectFile(repo, "master/2", "dir", "", nil)
	require.NoError(t, err)
	fmt.Printf("children: %+v", fileInfo.Children)
	require.Equal(t, 2, len(fileInfo.Children))
}

func TestNEWAPIListFileRF(t *testing.T) {
	t.Parallel()
	client, _ := getClientAndServer(t)
	repo := "test"
	require.NoError(t, client.CreateRepo(repo))

	fileContent := "foo\n"

	_, err := client.StartCommit(repo, "", "master")
	require.NoError(t, err)
	_, err = client.PutFile(repo, "master/0", "dir/1", strings.NewReader(fileContent))
	require.NoError(t, err)
	_, err = client.PutFile(repo, "master/0", "dir/2", strings.NewReader(fileContent))
	require.NoError(t, err)
	require.NoError(t, client.FinishCommit(repo, "master/0"))

	fileInfos, err := client.ListFile(repo, "master/0", "dir", "", nil, false)
	require.NoError(t, err)
	require.Equal(t, 2, len(fileInfos))

	_, err = client.StartCommit(repo, "", "master")
	require.NoError(t, err)
	_, err = client.PutFile(repo, "master/1", "dir/3", strings.NewReader(fileContent))
	require.NoError(t, err)
	require.NoError(t, client.FinishCommit(repo, "master/1"))

	fileInfos, err = client.ListFile(repo, "master/1", "dir", "", nil, false)
	require.NoError(t, err)
	require.Equal(t, 3, len(fileInfos))

	_, err = client.StartCommit(repo, "", "master")
	require.NoError(t, err)
	err = client.DeleteFile(repo, "master/2", "dir/2", false, "")
	require.NoError(t, err)
	require.NoError(t, client.FinishCommit(repo, "master/2"))

	fileInfos, err = client.ListFile(repo, "master/2", "dir", "", nil, false)
	require.NoError(t, err)
	require.Equal(t, 2, len(fileInfos))
}

func TestNEWAPIListFileRecurseRF(t *testing.T) {
	t.Parallel()
	client, _ := getClientAndServer(t)
	repo := "test"
	require.NoError(t, client.CreateRepo(repo))

	fileContent := "foo\n"

	_, err := client.StartCommit(repo, "", "master")
	require.NoError(t, err)
	_, err = client.PutFile(repo, "master/0", "dir/1", strings.NewReader(fileContent))
	require.NoError(t, err)
	_, err = client.PutFile(repo, "master/0", "dir/2", strings.NewReader(fileContent))
	require.NoError(t, err)
	require.NoError(t, client.FinishCommit(repo, "master/0"))

	fileInfos, err := client.ListFile(repo, "master/0", "dir", "", nil, true)
	require.NoError(t, err)
	require.Equal(t, 2, len(fileInfos))

	_, err = client.StartCommit(repo, "", "master")
	require.NoError(t, err)
	_, err = client.PutFile(repo, "master/1", "dir/3/foo", strings.NewReader(fileContent))
	require.NoError(t, err)
	_, err = client.PutFile(repo, "master/1", "dir/3/bar", strings.NewReader(fileContent))
	require.NoError(t, err)
	require.NoError(t, client.FinishCommit(repo, "master/1"))

	fileInfos, err = client.ListFile(repo, "master/1", "dir", "", nil, true)
	require.NoError(t, err)
	require.Equal(t, 3, len(fileInfos))
	require.Equal(t, int(fileInfos[2].SizeBytes), len(fileContent)*2)

	_, err = client.StartCommit(repo, "", "master")
	require.NoError(t, err)
	err = client.DeleteFile(repo, "master/2", "dir/3/bar", false, "")
	require.NoError(t, err)
	require.NoError(t, client.FinishCommit(repo, "master/2"))

	fileInfos, err = client.ListFile(repo, "master/2", "dir", "", nil, true)
	require.NoError(t, err)
	require.Equal(t, 3, len(fileInfos))
	require.Equal(t, int(fileInfos[2].SizeBytes), len(fileContent))
}

func TestNEWAPIPutFileTypeConflictRF(t *testing.T) {
	t.Parallel()
	client, _ := getClientAndServer(t)
	repo := "test"
	require.NoError(t, client.CreateRepo(repo))

	fileContent := "foo\n"

	_, err := client.StartCommit(repo, "", "master")
	require.NoError(t, err)
	_, err = client.PutFile(repo, "master", "dir/1", strings.NewReader(fileContent))
	require.NoError(t, err)
	require.NoError(t, client.FinishCommit(repo, "master"))

	_, err = client.StartCommit(repo, "", "master")
	require.NoError(t, err)
	_, err = client.PutFile(repo, "master", "dir", strings.NewReader(fileContent))
	require.YesError(t, err)
	require.NoError(t, client.FinishCommit(repo, "master"))
}

func TestRootDirectoryRF(t *testing.T) {
	t.Parallel()
	client, _ := getClientAndServer(t)
	repo := "test"
	require.NoError(t, client.CreateRepo(repo))

	fileContent := "foo\n"

	_, err := client.StartCommit(repo, "", "master")
	require.NoError(t, err)
	_, err = client.PutFile(repo, "master", "foo", strings.NewReader(fileContent))
	require.NoError(t, err)
	require.NoError(t, client.FinishCommit(repo, "master"))

	fileInfos, err := client.ListFile(repo, "master", "", "", nil, false)
	require.NoError(t, err)
	require.Equal(t, 1, len(fileInfos))
}

func TestSquashMergeSameFileRF(t *testing.T) {
	t.Parallel()
	client, _ := getClientAndServer(t)
	repo := "test"
	require.NoError(t, client.CreateRepo(repo))

	commitRoot, err := client.StartCommit(repo, "", "master")
	require.NoError(t, err)
	require.NoError(t, client.FinishCommit(repo, "master"))

	contentA1 := "foo1\n"
	_, err = client.StartCommit(repo, commitRoot.ID, "A")
	require.NoError(t, err)
	_, err = client.PutFile(repo, "A", "file", strings.NewReader(contentA1))
	require.NoError(t, err)
	require.NoError(t, client.FinishCommit(repo, "A"))

	contentA2 := "foo2\n"
	_, err = client.StartCommit(repo, "", "A")
	require.NoError(t, err)
	_, err = client.PutFile(repo, "A", "file", strings.NewReader(contentA2))
	require.NoError(t, err)
	require.NoError(t, client.FinishCommit(repo, "A"))

	contentB1 := "bar1\n"
	_, err = client.StartCommit(repo, commitRoot.ID, "B")
	require.NoError(t, err)
	_, err = client.PutFile(repo, "B", "file", strings.NewReader(contentB1))
	require.NoError(t, err)
	require.NoError(t, client.FinishCommit(repo, "B"))

	contentB2 := "bar2\n"
	_, err = client.StartCommit(repo, "", "B")
	require.NoError(t, err)
	_, err = client.PutFile(repo, "B", "file", strings.NewReader(contentB2))
	require.NoError(t, err)
	require.NoError(t, client.FinishCommit(repo, "B"))

	mergedCommits, err := client.Merge(repo, []string{"A", "B"}, "master", pfsclient.MergeStrategy_SQUASH)
	require.NoError(t, err)
	require.Equal(t, 1, len(mergedCommits))

	buffer := &bytes.Buffer{}
	require.NoError(t, client.GetFile(repo, mergedCommits[0].ID, "file", 0, 0, "", nil, buffer))
	// The ordering of commits within the same branch should be preserved
	require.EqualOneOf(t, []interface{}{contentA1 + contentA2 + contentB1 + contentB2, contentB1 + contentB2 + contentA1 + contentA2}, buffer.String())
}

func TestReplayMergeSameFileRF(t *testing.T) {
	t.Parallel()
	client, _ := getClientAndServer(t)
	repo := "test"
	require.NoError(t, client.CreateRepo(repo))

	commitRoot, err := client.StartCommit(repo, "", "master")
	require.NoError(t, err)
	require.NoError(t, client.FinishCommit(repo, "master"))

	contentA1 := "foo1\n"
	_, err = client.StartCommit(repo, commitRoot.ID, "A")
	require.NoError(t, err)
	_, err = client.PutFile(repo, "A", "file", strings.NewReader(contentA1))
	require.NoError(t, err)
	require.NoError(t, client.FinishCommit(repo, "A"))

	contentA2 := "foo2\n"
	_, err = client.StartCommit(repo, "", "A")
	require.NoError(t, err)
	_, err = client.PutFile(repo, "A", "file", strings.NewReader(contentA2))
	require.NoError(t, err)
	require.NoError(t, client.FinishCommit(repo, "A"))

	contentB1 := "bar1\n"
	_, err = client.StartCommit(repo, commitRoot.ID, "B")
	require.NoError(t, err)
	_, err = client.PutFile(repo, "B", "file", strings.NewReader(contentB1))
	require.NoError(t, err)
	require.NoError(t, client.FinishCommit(repo, "B"))

	contentB2 := "bar2\n"
	_, err = client.StartCommit(repo, "", "B")
	require.NoError(t, err)
	_, err = client.PutFile(repo, "B", "file", strings.NewReader(contentB2))
	require.NoError(t, err)
	require.NoError(t, client.FinishCommit(repo, "B"))

	mergedCommits, err := client.Merge(repo, []string{"A", "B"}, "master", pfsclient.MergeStrategy_REPLAY)
	require.NoError(t, err)
	require.Equal(t, 4, len(mergedCommits))

	buffer := &bytes.Buffer{}
	require.NoError(t, client.GetFile(repo, mergedCommits[3].ID, "file", 0, 0, "", nil, buffer))
	// The ordering of commits within the same branch should be preserved
	require.EqualOneOf(t, []interface{}{contentA1 + contentA2 + contentB1 + contentB2, contentB1 + contentB2 + contentA1 + contentA2}, buffer.String())
}

func TestSquashMergeDiffOrderingRF(t *testing.T) {
	t.Parallel()
	client, _ := getClientAndServer(t)
	repo := "test"
	require.NoError(t, client.CreateRepo(repo))

	commitRoot, err := client.StartCommit(repo, "", "master")
	require.NoError(t, err)
	require.NoError(t, client.FinishCommit(repo, "master"))

	contentA1 := "foo1\n"
	contentA2 := "foo2\n"
	_, err = client.StartCommit(repo, commitRoot.ID, "A")
	require.NoError(t, err)
	_, err = client.PutFile(repo, "A", "file", strings.NewReader(contentA1))
	require.NoError(t, err)
	_, err = client.PutFile(repo, "A", "file", strings.NewReader(contentA2))
	require.NoError(t, err)
	require.NoError(t, client.FinishCommit(repo, "A"))

	contentB1 := "bar1\n"
	_, err = client.StartCommit(repo, commitRoot.ID, "B")
	require.NoError(t, err)
	_, err = client.PutFile(repo, "B", "file", strings.NewReader(contentB1))
	require.NoError(t, err)
	require.NoError(t, client.FinishCommit(repo, "B"))

	mergedCommits, err := client.Merge(repo, []string{"A", "B"}, "master", pfsclient.MergeStrategy_SQUASH)
	require.NoError(t, err)
	require.Equal(t, 1, len(mergedCommits))

	buffer := &bytes.Buffer{}
	require.NoError(t, client.GetFile(repo, mergedCommits[0].ID, "file", 0, 0, "", nil, buffer))
	// The ordering of commits within the same branch should be preserved
	require.EqualOneOf(t, []interface{}{contentA1 + contentA2 + contentB1, contentB1 + contentA1 + contentA2}, buffer.String())
}

func TestReplayMergeDiffOrderingRF(t *testing.T) {
	t.Parallel()
	client, _ := getClientAndServer(t)
	repo := "test"
	require.NoError(t, client.CreateRepo(repo))

	commitRoot, err := client.StartCommit(repo, "", "master")
	require.NoError(t, err)
	require.NoError(t, client.FinishCommit(repo, "master"))

	contentA1 := "foo1\n"
	contentA2 := "foo2\n"
	_, err = client.StartCommit(repo, commitRoot.ID, "A")
	require.NoError(t, err)
	_, err = client.PutFile(repo, "A", "file", strings.NewReader(contentA1))
	require.NoError(t, err)
	_, err = client.PutFile(repo, "A", "file", strings.NewReader(contentA2))
	require.NoError(t, err)
	require.NoError(t, client.FinishCommit(repo, "A"))

	contentB1 := "bar1\n"
	_, err = client.StartCommit(repo, commitRoot.ID, "B")
	require.NoError(t, err)
	_, err = client.PutFile(repo, "B", "file", strings.NewReader(contentB1))
	require.NoError(t, err)
	require.NoError(t, client.FinishCommit(repo, "B"))

	mergedCommits, err := client.Merge(repo, []string{"A", "B"}, "master", pfsclient.MergeStrategy_REPLAY)
	require.NoError(t, err)
	require.Equal(t, 2, len(mergedCommits))

	buffer := &bytes.Buffer{}
	require.NoError(t, client.GetFile(repo, mergedCommits[1].ID, "file", 0, 0, "", nil, buffer))
	// The ordering of commits within the same branch should be preserved
	require.EqualOneOf(t, []interface{}{contentA1 + contentA2 + contentB1, contentB1 + contentA1 + contentA2}, buffer.String())
}

func TestReplayMergeBranchesRF(t *testing.T) {
	t.Parallel()
	client, _ := getClientAndServer(t)
	repo := "test"
	require.NoError(t, client.CreateRepo(repo))

	commitRoot, err := client.StartCommit(repo, "", "master")
	require.NoError(t, err)
	require.NoError(t, client.FinishCommit(repo, "master"))

	contentA1 := "foo1\n"
	_, err = client.StartCommit(repo, commitRoot.ID, "A")
	require.NoError(t, err)
	_, err = client.PutFile(repo, "A", "file", strings.NewReader(contentA1))
	require.NoError(t, err)
	require.NoError(t, client.FinishCommit(repo, "A"))

	contentA2 := "foo2\n"
	_, err = client.StartCommit(repo, "", "A")
	require.NoError(t, err)
	_, err = client.PutFile(repo, "A", "file", strings.NewReader(contentA2))
	require.NoError(t, err)
	require.NoError(t, client.FinishCommit(repo, "A"))

	contentA3 := "foo3\n"
	_, err = client.StartCommit(repo, "", "A")
	require.NoError(t, err)
	_, err = client.PutFile(repo, "A", "file", strings.NewReader(contentA3))
	require.NoError(t, err)
	require.NoError(t, client.FinishCommit(repo, "A"))

	contentB1 := "bar1\n"
	_, err = client.StartCommit(repo, commitRoot.ID, "B")
	require.NoError(t, err)
	_, err = client.PutFile(repo, "B", "file", strings.NewReader(contentB1))
	require.NoError(t, err)
	require.NoError(t, client.FinishCommit(repo, "B"))

	contentB2 := "bar2\n"
	_, err = client.StartCommit(repo, "", "B")
	require.NoError(t, err)
	_, err = client.PutFile(repo, "B", "file", strings.NewReader(contentB2))
	require.NoError(t, err)
	require.NoError(t, client.FinishCommit(repo, "B"))

	mergedCommits, err := client.Merge(repo, []string{"A"}, "B", pfsclient.MergeStrategy_REPLAY)
	require.NoError(t, err)
	require.Equal(t, 3, len(mergedCommits))

	buffer := &bytes.Buffer{}
	require.NoError(t, client.GetFile(repo, mergedCommits[2].ID, "file", 0, 0, "", nil, buffer))
	// The ordering of commits within the same branch should be preserved
	require.Equal(t, contentB1+contentB2+contentA1+contentA2+contentA3, buffer.String())
}

func TestReplayMergeMultipleFilesRF(t *testing.T) {
	t.Parallel()
	client, _ := getClientAndServer(t)
	repo := "test"
	require.NoError(t, client.CreateRepo(repo))

	commitRoot, err := client.StartCommit(repo, "", "master")
	require.NoError(t, err)
	require.NoError(t, client.FinishCommit(repo, "master"))

	contentA1 := "foo1\n"
	contentA2 := "foo2\n"
	_, err = client.StartCommit(repo, commitRoot.ID, "A")
	require.NoError(t, err)
	_, err = client.PutFile(repo, "A", "file1", strings.NewReader(contentA1))
	require.NoError(t, err)
	_, err = client.PutFile(repo, "A", "file2", strings.NewReader(contentA2))
	require.NoError(t, err)
	require.NoError(t, client.FinishCommit(repo, "A"))

	contentB1 := "bar1\n"
	contentB2 := "bar2\n"
	_, err = client.StartCommit(repo, commitRoot.ID, "B")
	require.NoError(t, err)
	_, err = client.PutFile(repo, "B", "file1", strings.NewReader(contentB1))
	require.NoError(t, err)
	_, err = client.PutFile(repo, "B", "file2", strings.NewReader(contentB2))
	require.NoError(t, err)
	require.NoError(t, client.FinishCommit(repo, "B"))

	mergedCommits, err := client.Merge(repo, []string{"A", "B"}, "master", pfsclient.MergeStrategy_REPLAY)
	require.NoError(t, err)
	require.Equal(t, 2, len(mergedCommits))

	buffer := &bytes.Buffer{}
	require.NoError(t, client.GetFile(repo, mergedCommits[1].ID, "file1", 0, 0, "", nil, buffer))
	require.EqualOneOf(t, []interface{}{contentB1 + contentA1, contentA1 + contentB1}, buffer.String())

	buffer.Reset()
	require.NoError(t, client.GetFile(repo, mergedCommits[1].ID, "file2", 0, 0, "", nil, buffer))
	require.EqualOneOf(t, []interface{}{contentB2 + contentA2, contentA2 + contentB2}, buffer.String())
}

func TestSquashMergeMultipleFilesRF(t *testing.T) {
	t.Parallel()
	client, _ := getClientAndServer(t)
	repo := "test"
	require.NoError(t, client.CreateRepo(repo))

	commitRoot, err := client.StartCommit(repo, "", "master")
	require.NoError(t, err)
	require.NoError(t, client.FinishCommit(repo, "master"))

	contentA1 := "foo1\n"
	_, err = client.StartCommit(repo, commitRoot.ID, "A")
	require.NoError(t, err)
	_, err = client.PutFile(repo, "A", "file1", strings.NewReader(contentA1))
	require.NoError(t, err)
	_, err = client.PutFile(repo, "A", "file2", strings.NewReader(contentA1))
	require.NoError(t, err)
	require.NoError(t, client.FinishCommit(repo, "A"))

	contentA2 := "foo2\n"
	_, err = client.StartCommit(repo, "", "A")
	require.NoError(t, err)
	_, err = client.PutFile(repo, "A", "file1", strings.NewReader(contentA2))
	require.NoError(t, err)
	_, err = client.PutFile(repo, "A", "file2", strings.NewReader(contentA2))
	require.NoError(t, err)
	require.NoError(t, client.FinishCommit(repo, "A"))

	contentB1 := "bar1\n"
	_, err = client.StartCommit(repo, commitRoot.ID, "B")
	require.NoError(t, err)
	_, err = client.PutFile(repo, "B", "file1", strings.NewReader(contentB1))
	require.NoError(t, err)
	_, err = client.PutFile(repo, "B", "file2", strings.NewReader(contentB1))
	require.NoError(t, err)
	require.NoError(t, client.FinishCommit(repo, "B"))

	mergedCommits, err := client.Merge(repo, []string{"A", "B"}, "master", pfsclient.MergeStrategy_SQUASH)
	require.NoError(t, err)
	require.Equal(t, 1, len(mergedCommits))

	buffer := &bytes.Buffer{}
	require.NoError(t, client.GetFile(repo, mergedCommits[0].ID, "file1", 0, 0, "", nil, buffer))
	require.EqualOneOf(t, []interface{}{contentA1 + contentA2 + contentB1, contentB1 + contentA1 + contentA2}, buffer.String())
}

func TestLeadingSlashesBreakThis(t *testing.T) {
	t.Parallel()
	client, _ := getClientAndServer(t)
	repo := "test"
	require.NoError(t, client.CreateRepo(repo))

	contentA1 := "foo1\n"
	contentA2 := "foo2\n"
	commit1, err := client.StartCommit(repo, "", "A")
	require.NoError(t, err)
	_, err = client.PutFile(repo, "A", "dir/file1", strings.NewReader(contentA1))
	require.NoError(t, err)
	_, err = client.PutFile(repo, "A", "dir/file2", strings.NewReader(contentA2))
	require.NoError(t, err)
	require.NoError(t, client.FinishCommit(repo, "A"))

	shard1 := &pfsclient.Shard{
		FileNumber:  0,
		FileModulus: 2,
	}
	fileInfos1, err := client.ListFile(repo, commit1.ID, "dir", "", shard1,
		false)
	require.NoError(t, err)
	shard2 := &pfsclient.Shard{
		FileNumber:  1,
		FileModulus: 2,
	}
	fileInfos2, err := client.ListFile(repo, commit1.ID, "dir", "", shard2, false)
	require.Equal(t, 2, len(fileInfos1)+len(fileInfos2))
}

func TestListFileWithFiltering(t *testing.T) {
	t.Parallel()
	client, _ := getClientAndServer(t)
	repo := "test"
	require.NoError(t, client.CreateRepo(repo))

	contentA1 := "foo1\n"
	contentA2 := "foo2\n"
	commit1, err := client.StartCommit(repo, "", "A")
	require.NoError(t, err)
	_, err = client.PutFile(repo, "A", "file1", strings.NewReader(contentA1))
	require.NoError(t, err)
	_, err = client.PutFile(repo, "A", "file2", strings.NewReader(contentA2))
	require.NoError(t, err)
	require.NoError(t, client.FinishCommit(repo, "A"))

	shard1 := &pfsclient.Shard{
		FileNumber:  0,
		FileModulus: 2,
	}
	fileInfos1, err := client.ListFile(repo, commit1.ID, "", "", shard1,
		false)
	require.NoError(t, err)
	shard2 := &pfsclient.Shard{
		FileNumber:  1,
		FileModulus: 2,
	}
	fileInfos2, err := client.ListFile(repo, commit1.ID, "", "", shard2, false)
	require.Equal(t, 2, len(fileInfos1)+len(fileInfos2))
}