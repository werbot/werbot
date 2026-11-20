@id("binding:019a0000-0000-7000-8000-000000000004")
permit (principal == Werbot::User::"019a0000-0000-7000-8000-000000000001", action, resource)
when { [Werbot::Resource::"019a0000-0000-7000-8000-000000000003"].contains(resource) };

@id("command:019a0000-0000-7000-8000-0000000000c1")
@message("No rm")
forbid (principal, action in [Werbot::Action::"ssh.exec"], resource)
when { principal in [Werbot::Team::"019a0000-0000-7000-8000-000000000002"] && [Werbot::Resource::"019a0000-0000-7000-8000-000000000003", Werbot::Resource::"019a0000-0000-7000-8000-000000000006"].contains(resource) && context.classes.containsAny(["destructive", "secrets"]) };

@id("command:019a0000-0000-7000-8000-0000000000c2")
@obligation("approval")
forbid (principal, action in [Werbot::Action::"ssh.exec"], resource)
when { principal in [Werbot::User::"019a0000-0000-7000-8000-000000000001", Werbot::User::"019a0000-0000-7000-8000-000000000005"] && [Werbot::Resource::"019a0000-0000-7000-8000-000000000003"].contains(resource) && context.classes.containsAny(["write"]) }
unless { context.approval_granted };

@id("blocklist")
forbid (principal, action, resource) when { context.ip_blocked };
