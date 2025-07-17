package testcmd

import (
	"testing"

	"github.com/speakeasy-api/speakeasy-client-sdk-go/v3/pkg/models/shared"
)

func TestIsBusinessTierOrAboveAccountType(t *testing.T) {
	tests := []struct {
		name        string
		accountType shared.AccountType
		expected    bool
	}{
		{
			name:        "Business account type should return true",
			accountType: shared.AccountTypeBusiness,
			expected:    true,
		},
		{
			name:        "Enterprise account type should return true",
			accountType: shared.AccountTypeEnterprise,
			expected:    true,
		},
		{
			name:        "Free account type should return false",
			accountType: shared.AccountTypeFree,
			expected:    false,
		},
		{
			name:        "OSS account type should return true",
			accountType: shared.AccountType("OSS"),
			expected:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsBusinessTierOrAboveAccountType(tt.accountType)
			if result != tt.expected {
				t.Errorf("IsBusinessTierOrAboveAccountType(%v) = %v, expected %v", tt.accountType, result, tt.expected)
			}
		})
	}
}

func TestCheckTestingAccountType(t *testing.T) {
	tests := []struct {
		name        string
		accountType shared.AccountType
		expected    bool
	}{
		{
			name:        "Business account type should allow testing",
			accountType: shared.AccountTypeBusiness,
			expected:    true,
		},
		{
			name:        "Enterprise account type should allow testing",
			accountType: shared.AccountTypeEnterprise,
			expected:    true,
		},
		{
			name:        "Free account type should not allow testing",
			accountType: shared.AccountTypeFree,
			expected:    false,
		},
		{
			name:        "OSS account type should allow testing",
			accountType: shared.AccountType("OSS"),
			expected:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CheckTestingAccountType(tt.accountType)
			if result != tt.expected {
				t.Errorf("CheckTestingAccountType(%v) = %v, expected %v", tt.accountType, result, tt.expected)
			}
		})
	}
}