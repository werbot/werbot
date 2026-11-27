@id("binding:019a0000-0000-7000-8000-000000000004")
permit (principal == Werbot::User::"019a0000-0000-7000-8000-000000000001", action, resource)
when { [Werbot::Resource::"019a0000-0000-7000-8000-000000000003"].contains(resource) };

@id("command:019a0000-0000-7000-8000-0000000000c1")
@message("No rm")
forbid (principal, action in [Werbot::Action::"ssh.exec", Werbot::Action::"postgres.query"], resource)
when { (principal in [Werbot::Team::"019a0000-0000-7000-8000-000000000002"] || (principal is Werbot::Agent && principal.sponsor in [Werbot::Team::"019a0000-0000-7000-8000-000000000002"])) && [Werbot::Resource::"019a0000-0000-7000-8000-000000000003", Werbot::Resource::"019a0000-0000-7000-8000-000000000006"].contains(resource) && context.classes.containsAny(["destructive", "secrets"]) };

@id("command:019a0000-0000-7000-8000-0000000000c2")
@obligation("approval")
forbid (principal, action in [Werbot::Action::"ssh.exec", Werbot::Action::"postgres.query"], resource)
when { (principal in [Werbot::User::"019a0000-0000-7000-8000-000000000001", Werbot::User::"019a0000-0000-7000-8000-000000000005"] || (principal is Werbot::Agent && principal.sponsor in [Werbot::User::"019a0000-0000-7000-8000-000000000001", Werbot::User::"019a0000-0000-7000-8000-000000000005"])) && [Werbot::Resource::"019a0000-0000-7000-8000-000000000003"].contains(resource) && context.classes.containsAny(["write"]) }
unless { context.approval_granted };

@id("command:019a0000-0000-7000-8000-0000000000c2:agent")
permit (principal is Werbot::Agent, action in [Werbot::Action::"ssh.exec", Werbot::Action::"postgres.query"], resource)
when { (principal is Werbot::Agent && principal.sponsor in [Werbot::User::"019a0000-0000-7000-8000-000000000001", Werbot::User::"019a0000-0000-7000-8000-000000000005"]) && [Werbot::Resource::"019a0000-0000-7000-8000-000000000003"].contains(resource) && ["write"].containsAll(context.classes) };

@id("command:019a0000-0000-7000-8000-0000000000c3")
permit (principal is Werbot::Agent, action in [Werbot::Action::"ssh.exec", Werbot::Action::"postgres.query"], resource)
when { (principal is Werbot::Agent && principal.sponsor in [Werbot::User::"019a0000-0000-7000-8000-000000000001"]) && [Werbot::Resource::"019a0000-0000-7000-8000-000000000003"].contains(resource) && ["read"].containsAll(context.classes) };

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

@id("agent:baseline:db_connect")
permit (principal is Werbot::Agent, action in [Werbot::Action::"postgres.connect"], resource)
when { context.kind == "stream" && resource has agent_readonly && resource.agent_readonly };

@id("agent:baseline:db_read")
permit (principal is Werbot::Agent, action in [Werbot::Action::"postgres.query"], resource)
when { ["read"].containsAll(context.classes) };

@id("agent:baseline:db_readonly")
forbid (principal is Werbot::Agent, action in [Werbot::Action::"postgres.connect", Werbot::Action::"postgres.query"], resource)
unless { resource has agent_readonly && resource.agent_readonly };

@id("agent:baseline:frozen")
forbid (principal is Werbot::Agent, action, resource)
when { principal.status == "frozen" };

