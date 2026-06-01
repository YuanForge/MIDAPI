package handler

import (
	"errors"
	"fmt"
	"github.com/go-playground/validator/v10"
	"strings"
)

// formatBindError 将 go-playground/validator 的原始错误转换为中文提示。
func formatBindError(err error) string {
	var ve validator.ValidationErrors
	if !errors.As(err, &ve) {
		return err.Error()
	}
	msgs := make([]string, 0, len(ve))
	for _, fe := range ve {
		field := fe.Field()
		tag := fe.Tag()
		param := fe.Param()
		switch field {
		case "Password":
			switch tag {
			case "min":
				msgs = append(msgs, fmt.Sprintf("密码长度不能少于 %s 位", param))
			case "max":
				msgs = append(msgs, fmt.Sprintf("密码长度不能超过 %s 位", param))
			default:
				msgs = append(msgs, "密码格式不正确")
			}
		case "Username":
			switch tag {
			case "min":
				msgs = append(msgs, fmt.Sprintf("用户名长度不能少于 %s 位", param))
			case "max":
				msgs = append(msgs, fmt.Sprintf("用户名长度不能超过 %s 位", param))
			default:
				msgs = append(msgs, "用户名格式不正确")
			}
		case "Email":
			msgs = append(msgs, "邮箱格式不正确")
		case "Code":
			msgs = append(msgs, "验证码不能为空")
		default:
			msgs = append(msgs, field+"不能为空或格式不正确")
		}
	}
	return strings.Join(msgs, "；")
}
