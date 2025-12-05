package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/Agyn-sandbox/gh-pr-review/internal/comments"
	"github.com/Agyn-sandbox/gh-pr-review/internal/resolver"
)

type commentsOptions struct {
	Repo string
	Pull int
}

func newCommentsCommand() *cobra.Command {
	opts := &commentsOptions{}

	cmd := &cobra.Command{
		Use:   "comments",
		Short: "Reply to pull request review comments",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := cmd.Help(); err != nil {
				return err
			}
			return errors.New("use 'gh pr-review comments reply' to respond to a review comment; run 'gh pr-review review report' to locate comment IDs")
		},
	}

	cmd.PersistentFlags().StringVarP(&opts.Repo, "repo", "R", "", "Repository in 'owner/repo' format")
	cmd.PersistentFlags().IntVar(&opts.Pull, "pr", 0, "Pull request number")

	cmd.AddCommand(newCommentsReplyCommand(opts))

	return cmd
}

func newCommentsReplyCommand(parent *commentsOptions) *cobra.Command {
	opts := &commentsReplyOptions{}

	cmd := &cobra.Command{
		Use:   "reply [<number> | <url> | <owner>/<repo>#<number>]",
		Short: "Reply to a pull request review comment",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				opts.Selector = args[0]
			}
			if opts.Repo == "" {
				opts.Repo = parent.Repo
			}
			if opts.Pull == 0 {
				opts.Pull = parent.Pull
			}
			return runCommentsReply(cmd, opts)
		},
	}

	cmd.Flags().StringVarP(&opts.Repo, "repo", "R", "", "Repository in 'owner/repo' format")
	cmd.Flags().IntVar(&opts.Pull, "pr", 0, "Pull request number")
	cmd.Flags().Int64Var(&opts.CommentID, "comment-id", 0, "Review comment identifier to reply to")
	cmd.Flags().StringVar(&opts.Body, "body", "", "Reply text")
	cmd.Flags().BoolVar(&opts.Concise, "concise", false, "Emit minimal reply payload { \"id\" }")
	_ = cmd.MarkFlagRequired("comment-id")
	_ = cmd.MarkFlagRequired("body")

	return cmd
}

type commentsReplyOptions struct {
	Repo      string
	Pull      int
	Selector  string
	CommentID int64
	Body      string
	Concise   bool
}

func runCommentsReply(cmd *cobra.Command, opts *commentsReplyOptions) error {
	selector, err := resolver.NormalizeSelector(opts.Selector, opts.Pull)
	if err != nil {
		return err
	}

	hostEnv := os.Getenv("GH_HOST")
	identity, err := resolver.Resolve(selector, opts.Repo, hostEnv)
	if err != nil {
		return err
	}

	service := comments.NewService(apiClientFactory(identity.Host))

	reply, err := service.Reply(identity, comments.ReplyOptions{
		CommentID: opts.CommentID,
		Body:      opts.Body,
	})
	if err != nil {
		return err
	}
	if opts.Concise {
		var minimal struct {
			ID int64 `json:"id"`
		}
		if err := json.Unmarshal(reply, &minimal); err != nil {
			return fmt.Errorf("parse reply payload: %w", err)
		}
		if minimal.ID == 0 {
			return errors.New("reply response missing id")
		}
		return encodeJSON(cmd, map[string]int64{"id": minimal.ID})
	}
	return encodeJSON(cmd, reply)
}
