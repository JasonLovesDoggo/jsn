package main

import (
	"github.com/turbopuffer/turbopuffer-go"
	"github.com/turbopuffer/turbopuffer-go/option"
)

func NewClient(profile Profile) *turbopuffer.Client {
	opts := make([]option.RequestOption, 0, 4)
	if profile.APIKey != "" {
		opts = append(opts, option.WithAPIKey(profile.APIKey))
	}
	if profile.Region != "" {
		opts = append(opts, option.WithRegion(profile.Region))
	}
	if profile.BaseURL != "" {
		opts = append(opts, option.WithBaseURL(profile.BaseURL))
	}
	if profile.Namespace != "" {
		opts = append(opts, option.WithDefaultNamespace(profile.Namespace))
	}
	client := turbopuffer.NewClient(opts...)
	return &client
}
