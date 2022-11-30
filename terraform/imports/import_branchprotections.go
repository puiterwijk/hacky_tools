package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"text/template"

	"github.com/Khan/genqlient/graphql"
	"golang.org/x/oauth2"

	"queries"
)

var branchProtectionTemplate = template.Must(template.New("repo").Funcs(templateFuncs).Parse(branchProtectionTemplateStr))

var templateFuncs = template.FuncMap{
	"listRepr": func(list []string) string {
		if len(list) == 0 {
			return "[]"
		}

		var b strings.Builder

		fmt.Fprintln(&b, "[")

		for _, item := range list {
			fmt.Fprintf(&b, "      \"%s\",", item)
			fmt.Fprintln(&b, "")
		}

		fmt.Fprint(&b, "    ]")

		return b.String()
	},
}

const branchProtectionTemplateStr = `
resource "github_branch_protection" "{{ .RepoOwner }}-{{ .RepoResourceName }}-{{ .ResourceName }}" {
  provider = github.{{ .RepoOwner }}
  repository_id = github_repository.{{ .RepoOwner }}-{{ .RepoResourceName }}.node_id

  pattern = "{{ .Pattern }}"

  enforce_admins                  = {{ .IsAdminEnforced }}
  require_signed_commits          = {{ .RequiresCommitSignatures }}
  required_linear_history         = {{ .RequiresLinearHistory }}
  require_conversation_resolution = {{ .RequiresConversationResolution }}
  push_restrictions               = {{ listRepr .BypassForcePushAllowances }}
  allows_deletions                = {{ .AllowsDeletions }}
  allows_force_pushes             = {{ .AllowsForcePushes }}
  blocks_creations                = {{ .BlocksCreations }}
{{ if .RequiresStatusChecks }}
  required_status_checks {
    strict   = {{ .RequiresStrictStatusChecks }}
    contexts = {{ listRepr .RequiredStatusCheckContexts }}
  }
{{ end }}
  required_pull_request_reviews {
    dismiss_stale_reviews           = {{ .DismissesStaleReviews }}
    restrict_dismissals             = {{ .RestrictsReviewDismissals }}
    dismissal_restrictions          = {{ listRepr .ReviewDismissalAllowances }}
    pull_request_bypassers          = {{ listRepr .BypassPullRequestAllowances }}
    require_code_owner_reviews      = {{ .RequiresCodeOwnerReviews }}
    required_approving_review_count = {{ .RequiredApprovingReviewCount }}
  }
}
`

type ruleInfo struct {
	RepoOwner        string
	RepoResourceName string
	ResourceName     string

	Pattern string

	AllowsDeletions   bool
	AllowsForcePushes bool
	BlocksCreations   bool
	// Unsupported
	BypassForcePushAllowances   []string
	BypassPullRequestAllowances []string
	DismissesStaleReviews       bool
	IsAdminEnforced             bool
	// Unsupported
	LockAllowsFetchAndMerge bool
	// Unsupported
	LockBranch bool
	// Unsupported
	RequireLastPushApproval      bool
	RequiredApprovingReviewCount int
	RequiredStatusCheckContexts  []string
	// Unsupported
	RequiredStatusChecks           interface{}
	RequiresApprovingReviews       bool
	RequiresCodeOwnerReviews       bool
	RequiresCommitSignatures       bool
	RequiresConversationResolution bool
	RequiresLinearHistory          bool
	RequiresStatusChecks           bool
	RequiresStrictStatusChecks     bool
	RestrictsPushes                bool
	RestrictsReviewDismissals      bool
	ReviewDismissalAllowances      []string
}

func (r *ruleInfo) Check() {
	if r.LockAllowsFetchAndMerge {
		panic("LockAllowsFetchAndMerge detected")
	}
	if r.LockBranch {
		panic("LockBranch detected")
	}
	if r.RequireLastPushApproval {
		//panic("RequireLastPushApproval detected")
	}
}

func actorToString(actor interface{}) string {
	outer := reflect.ValueOf(actor)
	if outer.Kind() != reflect.Struct {
		fmt.Println("outer.Kind: ", outer.Kind())
		panic("Incorrect type into actor")
	}
	inner := outer.FieldByName("Actor")
	if !inner.IsValid() {
		fmt.Println("Invalid inner: ", inner)
		panic("Invalid inner")
	}
	if inner.Kind() != reflect.Interface {
		fmt.Println("inner.Kind: ", inner.Kind())
		panic("Incorrect type for inner")
	}
	val := reflect.Indirect(reflect.ValueOf(inner.Interface()))
	if val.Kind() != reflect.Struct {
		fmt.Println("val.Kind: ", val.Kind())
		panic("Incorrect type for val")
	}

	if val.FieldByName("Organization").IsValid() {
		// This is a Team instance
		name := val.FieldByName("Name").String()
		org := val.FieldByName("Organization")
		return fmt.Sprintf("%s/%s", org, name)
	} else if val.FieldByName("Name").IsValid() {
		// This is an App instance
		name := val.FieldByName("Name")
		fmt.Println("Is app: name: ", name)
		panic("App encountered")
	} else if val.FieldByName("Login").IsValid() {
		// This is a User instance
		login := val.FieldByName("Login").String()
		return fmt.Sprintf("/%s", login)
	} else {
		panic("Unrecognized actor type")
	}
}

func actorsToList(actors interface{}) (out []string) {
	slice := reflect.ValueOf(actors)
	if slice.Kind() != reflect.Slice {
		fmt.Println("slice.Kind: ", slice.Kind())
		panic("Incorrect type into actors")
	}
	c := slice.Len()
	for i := 0; i < c; i++ {
		out = append(out, actorToString(slice.Index(i).Interface()))
	}
	return
}

func main() {
	if len(os.Args) < 3 {
		panic("Owner name and function needed")
	}
	token_cts, err := os.ReadFile("gh_token")
	if err != nil {
		panic("gh_token file not read")
	}
	token := strings.TrimSpace(string(token_cts))
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(ctx, ts)
	client := graphql.NewClient("https://api.github.com/graphql", tc)

	resp, err := queries.GetBranchProtections(ctx, client, os.Args[1])
	if err != nil {
		fmt.Println("Error performing query: ", err)
		panic("Query error")
	}
	for _, repo := range resp.RepositoryOwner.GetRepositories().Nodes {
		RepoResourceName := strings.ReplaceAll(repo.Name, ".", "_")
		for _, rule := range repo.BranchProtectionRules.Nodes {
			ResourceName := rule.Pattern
			ResourceName = strings.ReplaceAll(ResourceName, "*", "_wild_")
			ResourceName = strings.ReplaceAll(ResourceName, "/", "_slash_")
			ResourceName = strings.ReplaceAll(ResourceName, ".", "_")

			info := ruleInfo{
				RepoOwner:        os.Args[1],
				RepoResourceName: RepoResourceName,
				ResourceName:     ResourceName,

				Pattern:                        rule.Pattern,
				AllowsDeletions:                rule.AllowsDeletions,
				AllowsForcePushes:              rule.AllowsForcePushes,
				BlocksCreations:                rule.BlocksCreations,
				BypassForcePushAllowances:      actorsToList(rule.BypassForcePushAllowances.Nodes),
				BypassPullRequestAllowances:    actorsToList(rule.BypassPullRequestAllowances.Nodes),
				DismissesStaleReviews:          rule.DismissesStaleReviews,
				IsAdminEnforced:                rule.IsAdminEnforced,
				LockAllowsFetchAndMerge:        rule.LockAllowsFetchAndMerge,
				LockBranch:                     rule.LockBranch,
				RequireLastPushApproval:        rule.RequireLastPushApproval,
				RequiredApprovingReviewCount:   rule.RequiredApprovingReviewCount,
				RequiredStatusCheckContexts:    rule.RequiredStatusCheckContexts,
				RequiredStatusChecks:           rule.RequiredStatusChecks,
				RequiresApprovingReviews:       rule.RequiresApprovingReviews,
				RequiresCodeOwnerReviews:       rule.RequiresCodeOwnerReviews,
				RequiresCommitSignatures:       rule.RequiresCommitSignatures,
				RequiresConversationResolution: rule.RequiresConversationResolution,
				RequiresLinearHistory:          rule.RequiresLinearHistory,
				RequiresStatusChecks:           rule.RequiresStatusChecks,
				RequiresStrictStatusChecks:     rule.RequiresStrictStatusChecks,
				RestrictsPushes:                rule.RestrictsPushes,
				RestrictsReviewDismissals:      rule.RestrictsReviewDismissals,
				ReviewDismissalAllowances:      actorsToList(rule.ReviewDismissalAllowances.Nodes),
			}
			info.Check()
			if os.Args[2] == "generate" {
				if err := branchProtectionTemplate.Execute(os.Stdout, info); err != nil {
					fmt.Println("Error executing template: ", err)
					panic("Template execution failed")
				}
			} else if os.Args[2] == "import" {
				// github_branch_protection" "{{ .RepoOwner }}-{{ .RepoResourceName }}-{{ .ResourceName }}"
				FullResourceName := fmt.Sprintf(
					"github_branch_protection.%s-%s-%s",
					info.RepoOwner,
					info.RepoResourceName,
					info.ResourceName,
				)
				// terraform import -var-file local.tfvars github_branch_protection.terraform terraform:main
				cmd := exec.Command(
					"terraform",
					"import",
					"-var-file", "local.tfvars",
					"-input=false",
					"-lock=false",
					FullResourceName,
					fmt.Sprintf(
						"%s:%s",
						repo.Name,
						info.Pattern,
					),
				)
				cmd.Dir = ".."
				fmt.Println("Command: ", cmd)
				if err := cmd.Run(); err != nil {
					fmt.Println("Import of", FullResourceName, "failed: ", err)
					panic("Failed")
				}
				fmt.Println("Imported: ", FullResourceName)
			} else {
				panic("Function must be 'generate' or 'import'")
			}
		}
	}
}
