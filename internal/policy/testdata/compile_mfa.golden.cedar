@id("binding:019a0000-0000-7000-8000-000000000004")
permit (principal == Werbot::User::"019a0000-0000-7000-8000-000000000001", action, resource)
when { [Werbot::Resource::"019a0000-0000-7000-8000-000000000003", Werbot::Resource::"019a0000-0000-7000-8000-00000000000a"].contains(resource) };

@id("mfa:019a0000-0000-7000-8000-000000000101")
@obligation("mfa")
forbid (principal, action in [Werbot::Action::"rawterm.connect", Werbot::Action::"ssh.connect"], resource)
when { principal in [Werbot::Team::"019a0000-0000-7000-8000-000000000002"] && [Werbot::Resource::"019a0000-0000-7000-8000-000000000003", Werbot::Resource::"019a0000-0000-7000-8000-00000000000a"].contains(resource) }
unless { context.mfa_age < 60 && (true || context.mfa_method == "webauthn") };

@id("mfa:019a0000-0000-7000-8000-000000000102")
@obligation("mfa")
forbid (principal, action in [Werbot::Action::"ssh.connect"], resource)
when { principal in [Werbot::User::"019a0000-0000-7000-8000-000000000001"] && [Werbot::Resource::"019a0000-0000-7000-8000-000000000003"].contains(resource) }
unless { context.mfa_age < 300 && (false || context.mfa_method == "webauthn") };

@id("blocklist")
forbid (principal, action, resource) when { context.ip_blocked };

@id("agent:baseline:connect")
permit (principal is Werbot::Agent, action in [Werbot::Action::"ssh.connect"], resource)
when { context.kind == "exec" };

@id("agent:baseline:read")
permit (principal is Werbot::Agent, action in [Werbot::Action::"ssh.exec"], resource)
when { ["read"].containsAll(context.classes) };

@id("agent:baseline:dangerous")
forbid (principal is Werbot::Agent, action, resource)
when { context.classes.containsAny(["destructive", "secrets", "egress"]) };

@id("agent:baseline:terminal")
forbid (principal is Werbot::Agent, action, resource)
when { context.kind == "terminal" };

@id("agent:baseline:sftp")
forbid (principal is Werbot::Agent, action in [Werbot::Action::"sftpy.sftp"], resource);

@id("agent:baseline:remote_forward")
forbid (principal is Werbot::Agent, action in [Werbot::Action::"ssh.remote_forward"], resource);

@id("agent:baseline:db_readonly")
forbid (principal is Werbot::Agent, action in [Werbot::Action::"postgres.connect"], resource);

@id("agent:baseline:frozen")
forbid (principal is Werbot::Agent, action, resource)
when { principal.status == "frozen" };

