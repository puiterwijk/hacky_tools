package main

import (
  "fmt"
  "os"
  "context"
  "text/template"
  "strings"

  "github.com/google/go-github/v48/github"
)

const repoTemplateStr = `
resource "github_repository" "{{ .Info.Owner.Login }}-{{ .ResourceName }}" {
  provider = github.{{ .Info.Owner.Login }}

  lifecycle {
    prevent_destroy = true
  }
  archive_on_destroy = local.github_policy.archive_on_destroy

  name         = "{{ .Info.Name }}"
  description  = "{{ if .Info.Description }}{{ .Info.Description }}{{ end }}"
  homepage_url = "{{ if .Info.Homepage }}{{ .Info.Homepage }}{{ end }}"

  visibility    = local.github_policy.visibility
  has_issues    = local.github_policy.has_issues
  has_projects  = local.github_policy.has_projects
  has_wiki      = local.github_policy.has_wiki
  has_downloads = local.github_policy.has_downloads

  allow_merge_commit = local.github_policy.allow_merge_commit
  allow_squash_merge = local.github_policy.allow_squash_merge
  allow_rebase_merge = local.github_policy.allow_rebase_merge
  allow_auto_merge   = local.github_policy.allow_auto_merge

  squash_merge_commit_title   = local.github_policy.squash_merge_commit_title
  squash_merge_commit_message = local.github_policy.squash_merge_commit_message
  merge_commit_title          = local.github_policy.merge_commit_title
  merge_commit_message        = local.github_policy.merge_commit_message

  delete_branch_on_merge = local.github_policy.delete_branch_on_merge
  allow_update_branch    = local.github_policy.allow_update_branch

  auto_init          = local.github_policy.creation.auto_init
  gitignore_template = local.github_policy.creation.gitignore_template
  license_template   = local.github_policy.creation.license_template

  security_and_analysis {
    advanced_security {
      status = local.github_policy.security_and_analysis.advanced_security
    }
    secret_scanning {
      status = local.github_policy.security_and_analysis.secret_scanning
    }
    secret_scanning_push_protection {
      status = local.github_policy.security_and_analysis.secret_scanning_push_protection
    }
  }

  topics = local.github_policy.topics

  vulnerability_alerts                    = local.github_policy.vulnerability_alerts
  ignore_vulnerability_alerts_during_read = local.github_policy.ignore_vulnerability_alerts_during_read
}
`
var repoTemplate = template.Must(template.New("repo").Parse(repoTemplateStr))

type repoInfo struct {
    ResourceName string
    Info *github.Repository
}

func main() {
    if len(os.Args) != 2 {
        panic("Owner name needed")
    }
    ctx := context.Background()

    client := github.NewClient(nil)
    repoClient := client.Repositories
    repos, _, err := repoClient.List(ctx, os.Args[1], nil)
    if err != nil {
      fmt.Println("Error getting repos: ", err)
      panic("done")
    }
    for _, repo := range repos {
      info := repoInfo {
          ResourceName: strings.ReplaceAll(*repo.Name, ".", "_"),
          Info: repo,
      }
      if err := repoTemplate.Execute(os.Stdout, info); err != nil {
        fmt.Println("Error executing template: ", err)
        panic("Errored")
      }
    }
}
