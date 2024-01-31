package model

import (
	"github.com/spf13/cobra"
)

type Flag interface {
	init(cmd *cobra.Command) error
	getName() string
}

type StringFlag struct {
	Name, Shorthand, Description string
	Required, Hidden             bool
	DefaultValue                 string
}

func (f StringFlag) init(cmd *cobra.Command) error {
	cmd.Flags().StringP(f.Name, f.Shorthand, f.DefaultValue, f.Description)
	if err := setRequiredAndHidden(cmd, f.Name, f.Required, f.Hidden); err != nil {
		return err
	}
	return nil
}

func (f StringFlag) getName() string {
	return f.Name
}

type BooleanFlag struct {
	Name, Shorthand, Description string
	Required, Hidden             bool
	DefaultValue                 bool
}

func (f BooleanFlag) init(cmd *cobra.Command) error {
	cmd.Flags().BoolP(f.Name, f.Shorthand, f.DefaultValue, f.Description)
	if err := setRequiredAndHidden(cmd, f.Name, f.Required, f.Hidden); err != nil {
		return err
	}
	return nil
}

func (f BooleanFlag) getName() string {
	return f.Name
}

type IntFlag struct {
	Name, Shorthand, Description string
	Required, Hidden             bool
	DefaultValue                 int
}

func (f IntFlag) init(cmd *cobra.Command) error {
	cmd.Flags().IntP(f.Name, f.Shorthand, f.DefaultValue, f.Description)
	if err := setRequiredAndHidden(cmd, f.Name, f.Required, f.Hidden); err != nil {
		return err
	}

	return nil
}

func (f IntFlag) getName() string {
	return f.Name
}

func setRequiredAndHidden(cmd *cobra.Command, name string, required, hidden bool) error {
	if required {
		if err := cmd.MarkFlagRequired(name); err != nil {
			return err
		}
	}
	if hidden {
		if err := cmd.Flags().MarkHidden(name); err != nil {
			return err
		}
	}

	return nil
}

// Verify that the flag types implement the Flag interface
var _ = []Flag{
	&StringFlag{},
	&BooleanFlag{},
	&IntFlag{},
}
