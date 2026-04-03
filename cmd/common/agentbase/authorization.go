package agentbase

import (
	"encoding/json"
	"fmt"
	"strings"

	"uap"
)

type ToolPolicyMode string

const (
	ToolPolicyModeMatchAccount ToolPolicyMode = "match_account"
	ToolPolicyModeMatchOwner   ToolPolicyMode = "match_owner"
	ToolPolicyModeCustom       ToolPolicyMode = "custom"
)

// AuthorizationContext tool_call 授权上下文。
type AuthorizationContext struct {
	Message             *uap.Message
	Payload             *uap.ToolCallPayload
	ToolName            string
	RequestID           string
	FromAgentID         string
	AuthenticatedUser   string
	Arguments           map[string]interface{}
	Account             string
	ResourceID          string
	OwnershipResolver   OwnershipResolver
	OwnershipResourceID string
}

// AuthorizationDecision 授权决策。
type AuthorizationDecision struct {
	Allow  bool
	Error  string
	Reason string
}

// OwnershipResolver 按资源 ID 解析 owner。
type OwnershipResolver interface {
	ResolveOwner(ctx *AuthorizationContext) (owner string, found bool, err error)
}

// OwnershipResolverFunc 函数式 resolver。
type OwnershipResolverFunc func(ctx *AuthorizationContext) (owner string, found bool, err error)

func (f OwnershipResolverFunc) ResolveOwner(ctx *AuthorizationContext) (owner string, found bool, err error) {
	return f(ctx)
}

// Authorizer 统一授权入口。
type Authorizer interface {
	AuthorizeToolCall(ctx *AuthorizationContext) AuthorizationDecision
}

// AuthorizerFunc 函数式 Authorizer。
type AuthorizerFunc func(ctx *AuthorizationContext) AuthorizationDecision

func (f AuthorizerFunc) AuthorizeToolCall(ctx *AuthorizationContext) AuthorizationDecision {
	return f(ctx)
}

// ToolPolicy 单工具授权策略。
type ToolPolicy struct {
	Mode                     ToolPolicyMode
	AccountArg               string
	ResourceIDArg            string
	RequireAuthenticatedUser bool
	OwnershipResolver        OwnershipResolver
	Custom                   AuthorizerFunc
}

// ToolAuthorizer 按工具名执行声明式授权。
type ToolAuthorizer struct {
	Policies map[string]ToolPolicy
}

func NewToolAuthorizer(policies map[string]ToolPolicy) *ToolAuthorizer {
	return &ToolAuthorizer{Policies: policies}
}

func (a *ToolAuthorizer) AuthorizeToolCall(ctx *AuthorizationContext) AuthorizationDecision {
	if a == nil || ctx == nil {
		return AllowDecision()
	}
	policy, ok := a.Policies[ctx.ToolName]
	if !ok {
		return AllowDecision()
	}
	if policy.RequireAuthenticatedUser && strings.TrimSpace(ctx.AuthenticatedUser) == "" {
		return DenyDecision("权限拒绝：缺少认证用户", "missing_authenticated_user")
	}

	accountArg := strings.TrimSpace(policy.AccountArg)
	if accountArg == "" {
		accountArg = "account"
	}
	ctx.Account = stringValue(ctx.Arguments[accountArg])
	ctx.ResourceID = stringValue(ctx.Arguments[strings.TrimSpace(policy.ResourceIDArg)])
	ctx.OwnershipResolver = policy.OwnershipResolver
	ctx.OwnershipResourceID = ctx.ResourceID

	switch policy.Mode {
	case "", ToolPolicyModeMatchAccount:
		if ctx.AuthenticatedUser == "" || ctx.Account == "" {
			return AllowDecision()
		}
		if ctx.AuthenticatedUser != ctx.Account {
			return DenyDecision(
				fmt.Sprintf("权限拒绝：用户 %s 无权访问账户 %s", ctx.AuthenticatedUser, ctx.Account),
				"account_mismatch",
			)
		}
		return AllowDecision()
	case ToolPolicyModeMatchOwner:
		if policy.OwnershipResolver == nil {
			return DenyDecision("权限配置错误：缺少资源所有权解析器", "missing_ownership_resolver")
		}
		owner, found, err := policy.OwnershipResolver.ResolveOwner(ctx)
		if err != nil {
			return DenyDecision(fmt.Sprintf("权限校验失败：%v", err), "ownership_resolve_error")
		}
		if !found || strings.TrimSpace(owner) == "" {
			return DenyDecision("权限校验失败：资源不存在或未找到创建者", "ownership_not_found")
		}
		if strings.TrimSpace(ctx.AuthenticatedUser) == "" {
			return AllowDecision()
		}
		if strings.TrimSpace(owner) != strings.TrimSpace(ctx.AuthenticatedUser) {
			return DenyDecision("权限拒绝：只能操作自己创建的资源", "owner_mismatch")
		}
		return AllowDecision()
	case ToolPolicyModeCustom:
		if policy.Custom == nil {
			return AllowDecision()
		}
		return normalizeDecision(policy.Custom(ctx))
	default:
		return DenyDecision(fmt.Sprintf("权限配置错误：未知策略模式 %s", policy.Mode), "unknown_policy_mode")
	}
}

func AllowDecision() AuthorizationDecision {
	return AuthorizationDecision{Allow: true}
}

func DenyDecision(errMsg, reason string) AuthorizationDecision {
	return AuthorizationDecision{
		Allow:  false,
		Error:  strings.TrimSpace(errMsg),
		Reason: strings.TrimSpace(reason),
	}
}

func normalizeDecision(decision AuthorizationDecision) AuthorizationDecision {
	if decision.Allow {
		return decision
	}
	if strings.TrimSpace(decision.Error) == "" {
		decision.Error = "权限拒绝"
	}
	return decision
}

func buildAuthorizationContext(msg *uap.Message) (*AuthorizationContext, error) {
	var payload uap.ToolCallPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return nil, fmt.Errorf("invalid payload: %w", err)
	}

	args := make(map[string]interface{})
	if len(payload.Arguments) > 0 {
		if err := json.Unmarshal(payload.Arguments, &args); err != nil {
			return nil, fmt.Errorf("invalid arguments: %w", err)
		}
	}

	return &AuthorizationContext{
		Message:           msg,
		Payload:           &payload,
		ToolName:          payload.ToolName,
		RequestID:         msg.ID,
		FromAgentID:       msg.From,
		AuthenticatedUser: strings.TrimSpace(payload.AuthenticatedUser),
		Arguments:         args,
	}, nil
}

func stringValue(v interface{}) string {
	s, _ := v.(string)
	return strings.TrimSpace(s)
}
