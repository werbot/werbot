//https://github.com/iDevoid/bitsum/blob/main/internal/handler/rest/coins_test.go

package middleware

/*
func Test_GetUserParametersFromCtx(t *testing.T) {
	t.Parallel()

	app := fiber.New()

	tests := []struct {
		name string
		args *fiber.Ctx
		want *pb.UserParameters
	}{
		{
			name: "test",
			args: func() *fiber.Ctx {
				ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
				//ctx.Locals("user")
				return ctx
			}(),
			want: &pb.UserParameters{
				UserRole: pb.RoleUser(pb.RoleUser_ROLE_USER_UNSPECIFIED),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetUserParametersFromCtx(tt.args)
			assert.Equal(t, tt.want, got)
		})
	}
}
*/
