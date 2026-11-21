@id("binding:019a0000-0000-7000-8000-000000000004")
permit (principal == Werbot::User::"019a0000-0000-7000-8000-000000000001", action, resource)
when { [Werbot::Resource::"019a0000-0000-7000-8000-000000000003", Werbot::Resource::"019a0000-0000-7000-8000-00000000000a"].contains(resource) };

@id("path:019a0000-0000-7000-8000-0000000000e1")
@message("No /etc")
forbid (principal, action in [Werbot::Action::"ssh.exec"], resource)
when { principal in [Werbot::Team::"019a0000-0000-7000-8000-000000000002"] && [Werbot::Resource::"019a0000-0000-7000-8000-000000000003"].contains(resource) && context.path_hits.contains("019a0000-0000-7000-8000-0000000000e1") };

@id("path:019a0000-0000-7000-8000-0000000000e2")
@obligation("approval")
forbid (principal, action in [Werbot::Action::"ssh.exec"], resource)
when { principal in [Werbot::User::"019a0000-0000-7000-8000-000000000001", Werbot::User::"019a0000-0000-7000-8000-000000000005"] && [Werbot::Resource::"019a0000-0000-7000-8000-000000000003"].contains(resource) && context.path_hits.contains("019a0000-0000-7000-8000-0000000000e2") }
unless { context.approval_granted };

@id("path:019a0000-0000-7000-8000-0000000000e3")
@message("path outside allowed folders")
forbid (principal, action in [Werbot::Action::"ssh.exec"], resource)
when { principal is Werbot::User && [Werbot::Resource::"019a0000-0000-7000-8000-000000000003"].contains(resource) && context.path_outside.contains("019a0000-0000-7000-8000-0000000000e3") };

@id("terminal:019a0000-0000-7000-8000-0000000000f1")
@obligation("console")
forbid (principal, action in [Werbot::Action::"ssh.shell"], resource)
when { principal in [Werbot::User::"019a0000-0000-7000-8000-000000000001"] && [Werbot::Resource::"019a0000-0000-7000-8000-000000000003", Werbot::Resource::"019a0000-0000-7000-8000-00000000000a"].contains(resource) }
unless { context.console };

@id("terminal:019a0000-0000-7000-8000-0000000000f1:deny")
forbid (principal, action in [Werbot::Action::"rawterm.shell"], resource)
when { principal in [Werbot::User::"019a0000-0000-7000-8000-000000000001"] && [Werbot::Resource::"019a0000-0000-7000-8000-000000000003", Werbot::Resource::"019a0000-0000-7000-8000-00000000000a"].contains(resource) };

@id("terminal:019a0000-0000-7000-8000-0000000000f2")
forbid (principal, action in [Werbot::Action::"rawterm.shell", Werbot::Action::"ssh.shell"], resource)
when { principal in [Werbot::Team::"019a0000-0000-7000-8000-000000000002"] && [Werbot::Resource::"019a0000-0000-7000-8000-000000000003", Werbot::Resource::"019a0000-0000-7000-8000-00000000000a"].contains(resource) };

@id("blocklist")
forbid (principal, action, resource) when { context.ip_blocked };
