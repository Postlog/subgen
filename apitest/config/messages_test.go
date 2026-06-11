//go:build apitest

package config_test

import "github.com/postlog/subgen/internal/handlers/config_save"

// User-facing messages the config endpoints return. Black-box tests assert the exact text
// subgen produces — but rather than re-stating it here, these alias the exported config_save
// handler constants, so the text lives in exactly one place. (The provider-check probe has
// its own messages, asserted directly against provider_check.Msg* in provider_check_test.go.)
const (
	// mihomo-config save validation (config_save handler ← mihomo sentinels).
	msgMatchNotLast    = config_save.MsgMatchNotLast
	msgGroupNameTaken  = config_save.MsgGroupNameTaken
	msgGroupCycle      = config_save.MsgGroupCycle
	msgGroupRefRange   = config_save.MsgGroupRefRange
	msgProviderNameReq = config_save.MsgProviderNameEmpty
	msgProviderNameDup = config_save.MsgProviderNameTaken
	msgRuleSetUnknown  = config_save.MsgRuleSetUnknownProv
	msgGeneratedKey    = config_save.MsgGeneratedKey
	msgBaseYAMLInvalid = config_save.MsgBaseYAMLInvalid
	msgRuleValueReq    = config_save.MsgRuleValueReq
	msgGroupNoMembers  = config_save.MsgGroupNoMembers

	// Save success.
	msgSaved = config_save.MsgSaved
)
