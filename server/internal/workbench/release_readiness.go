package workbench

import (
	"net/url"
	"sort"
	"strings"
)

type ReleaseRepository struct {
	ID     string
	URL    string
	Branch string
}

type ReleasePullRequest struct {
	IssueID        string
	ID             string
	RepoURL        string
	URL            string
	State          string
	BaseBranch     string
	HeadSHA        string
	MergeCommitSHA string
}

type ReleaseMerge struct {
	RepoID        string `json:"repo_id"`
	PullRequestID string `json:"pull_request_id"`
	URL           string `json:"url"`
	BaseBranch    string `json:"base_branch"`
	SourceCommit  string `json:"source_commit"`
	MergeCommit   string `json:"merge_commit"`
}

type ReleaseBlocker struct {
	Code   string `json:"code"`
	RepoID string `json:"repo_id,omitempty"`
}

type ReleaseReadiness struct {
	Revision int64            `json:"revision"`
	Blockers []ReleaseBlocker `json:"blockers"`
	Merges   []ReleaseMerge   `json:"merges"`
}

func repositoryIdentity(raw string) string {
	raw = strings.TrimSpace(raw)
	if !strings.Contains(raw, "://") {
		user, path, ok := strings.Cut(raw, ":")
		if !ok || !strings.Contains(user, "@") {
			return ""
		}
		raw = "ssh://" + user + "/" + path
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Hostname() == "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return ""
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" && parsed.Scheme != "ssh" {
		return ""
	}
	host := strings.ToLower(parsed.Host)
	if parsed.Scheme == "ssh" {
		// SSH clone ports identify a transport, while webhook identities use the web origin.
		if parsed.Port() != "" && parsed.Port() != "22" {
			return ""
		}
		host = strings.ToLower(parsed.Hostname())
	}
	path := strings.TrimSuffix(strings.TrimSuffix(parsed.Path, "/"), ".git")
	if path == "" || path == "/" {
		return ""
	}
	return host + path
}

// Release readiness binds the accepted source revision to a confirmed merge in each repository.
func CheckReleaseReadiness(parentID, status string, criteria Requirements, repositories []ReleaseRepository, prs []ReleasePullRequest, deliveryReady bool) ReleaseReadiness {
	result := ReleaseReadiness{Blockers: []ReleaseBlocker{}, Merges: []ReleaseMerge{}}
	if status != "done" || criteria.Gate.Accept == nil || criteria.Review == nil || criteria.Review.Action != "accept" || criteria.Plan == nil || !deliveryReady {
		result.Blockers = append(result.Blockers, ReleaseBlocker{Code: "acceptance"})
		return result
	}
	sources := map[string]map[string]string{}
	for _, issueID := range criteria.Plan.Issues {
		evidence := criteria.Delivery[issueID]
		if evidence.RepoID == "" {
			continue
		}
		if sources[evidence.RepoID] == nil {
			sources[evidence.RepoID] = map[string]string{}
		}
		sources[evidence.RepoID][evidence.IssueID] = evidence.Commit
	}
	if len(sources) == 0 {
		result.Blockers = append(result.Blockers, ReleaseBlocker{Code: "repository_missing"})
		return result
	}
	byID := map[string]ReleaseRepository{}
	for _, repo := range repositories {
		byID[repo.ID] = repo
	}
	ids := make([]string, 0, len(sources))
	for id := range sources {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, repoID := range ids {
		repo := byID[repoID]
		identity := repositoryIdentity(repo.URL)
		if identity == "" || repo.Branch == "" {
			result.Blockers = append(result.Blockers, ReleaseBlocker{Code: "repository_missing", RepoID: repoID})
			continue
		}
		matched := map[string]ReleasePullRequest{}
		for _, pr := range prs {
			if repositoryIdentity(pr.RepoURL) != identity || pr.BaseBranch != repo.Branch || pr.State != "merged" || !sourceCommit.MatchString(pr.MergeCommitSHA) {
				continue
			}
			_, childLinked := sources[repoID][pr.IssueID]
			if pr.IssueID != parentID && !childLinked {
				continue
			}
			sameSource := true
			for _, commit := range sources[repoID] {
				if !sourceCommit.MatchString(commit) || !strings.EqualFold(commit, pr.HeadSHA) {
					sameSource = false
					break
				}
			}
			if sameSource {
				matched[pr.ID] = pr
			}
		}
		if len(matched) != 1 {
			code := "merge_missing"
			if len(matched) > 1 {
				code = "merge_ambiguous"
			}
			result.Blockers = append(result.Blockers, ReleaseBlocker{Code: code, RepoID: repoID})
			continue
		}
		for _, pr := range matched {
			result.Merges = append(result.Merges, ReleaseMerge{RepoID: repoID, PullRequestID: pr.ID, URL: pr.URL, BaseBranch: pr.BaseBranch, SourceCommit: pr.HeadSHA, MergeCommit: pr.MergeCommitSHA})
		}
	}
	return result
}
