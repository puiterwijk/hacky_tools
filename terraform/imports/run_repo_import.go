package main

import (
	"fmt"
	"os/exec"
	"os"
	"context"
	"strings"

	"github.com/google/go-github/v48/github"
)

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
		ResourceName := fmt.Sprintf("github_repository.%s-%s", *repo.Owner.Login, strings.ReplaceAll(*repo.Name, ".", "_"))

		// terraform import -var-file local.tfvars github_repository.profianinc-infra-terraform infra-terraform
		cmd := exec.Command(
			"terraform",
			"import",
			"-var-file", "local.tfvars",
			ResourceName,
			*repo.Name,
		)
		cmd.Dir = ".."
		if err := cmd.Run(); err != nil {
			fmt.Println("Import of", ResourceName, "failed: ", err)
			panic("Failed")
		}
		fmt.Println("Imported: ", ResourceName)
	}
}
