package agentbase

import "testing"

func TestToolAuthorizerMatchAccount(t *testing.T) {
	authorizer := NewToolAuthorizer(map[string]ToolPolicy{
		"cronCreateTask": {
			Mode:                     ToolPolicyModeMatchAccount,
			AccountArg:               "account",
			RequireAuthenticatedUser: true,
		},
	})

	tests := []struct {
		name      string
		user      string
		account   string
		allow     bool
		reason    string
		expectErr string
	}{
		{name: "match", user: "ztt", account: "ztt", allow: true},
		{name: "mismatch", user: "ztt", account: "other", allow: false, reason: "account_mismatch", expectErr: "权限拒绝：用户 ztt 无权访问账户 other"},
		{name: "missing user", user: "", account: "ztt", allow: false, reason: "missing_authenticated_user", expectErr: "权限拒绝：缺少认证用户"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decision := authorizer.AuthorizeToolCall(&AuthorizationContext{
				ToolName:          "cronCreateTask",
				AuthenticatedUser: tt.user,
				Arguments:         map[string]interface{}{"account": tt.account},
			})
			if decision.Allow != tt.allow {
				t.Fatalf("Allow=%v want %v", decision.Allow, tt.allow)
			}
			if decision.Reason != tt.reason {
				t.Fatalf("Reason=%q want %q", decision.Reason, tt.reason)
			}
			if decision.Error != tt.expectErr {
				t.Fatalf("Error=%q want %q", decision.Error, tt.expectErr)
			}
		})
	}
}

func TestToolAuthorizerMatchOwner(t *testing.T) {
	authorizer := NewToolAuthorizer(map[string]ToolPolicy{
		"cronDeleteTask": {
			Mode:                     ToolPolicyModeMatchOwner,
			ResourceIDArg:            "task_id",
			RequireAuthenticatedUser: true,
			OwnershipResolver: OwnershipResolverFunc(func(ctx *AuthorizationContext) (string, bool, error) {
				switch ctx.ResourceID {
				case "task-1":
					return "ztt", true, nil
				case "missing":
					return "", false, nil
				default:
					return "", false, nil
				}
			}),
		},
	})

	tests := []struct {
		name      string
		user      string
		taskID    string
		allow     bool
		reason    string
		expectErr string
	}{
		{name: "owner match", user: "ztt", taskID: "task-1", allow: true},
		{name: "owner mismatch", user: "other", taskID: "task-1", allow: false, reason: "owner_mismatch", expectErr: "权限拒绝：只能操作自己创建的资源"},
		{name: "missing resource", user: "ztt", taskID: "missing", allow: false, reason: "ownership_not_found", expectErr: "权限校验失败：资源不存在或未找到创建者"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decision := authorizer.AuthorizeToolCall(&AuthorizationContext{
				ToolName:          "cronDeleteTask",
				AuthenticatedUser: tt.user,
				Arguments:         map[string]interface{}{"task_id": tt.taskID},
			})
			if decision.Allow != tt.allow {
				t.Fatalf("Allow=%v want %v", decision.Allow, tt.allow)
			}
			if decision.Reason != tt.reason {
				t.Fatalf("Reason=%q want %q", decision.Reason, tt.reason)
			}
			if decision.Error != tt.expectErr {
				t.Fatalf("Error=%q want %q", decision.Error, tt.expectErr)
			}
		})
	}
}
