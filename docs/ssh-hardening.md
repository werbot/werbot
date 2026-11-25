# Hardening SSH targets

Werbot decides who may open a resource, which commands run and which folders they touch ([policies.md](policies.md)). These rules read the command text, so they are the second line of defence and the audit trail. The first line is the permissions of the account Werbot logs in with on the host: whatever that account can do, a script or a program it runs can do too ([honest limits](policies.md#honest-limits)).

## One account per access level

Do not let every resource log in as one shared account, and never as `root`.

- Create one Unix account per access level, for example `webro` (reads the application and its logs) and `deploy` (writes releases).
- Add one resource per account on the same host: `web-ro` with the login `webro`, `web-deploy` with the login `deploy`, each with its own credential.
- Bind people to the resource of the level they need. A JIT request for `web-deploy` then means "write access", and the audit log shows which level was used.
- Set `PermitRootLogin no` in `sshd_config`.

## Folder permissions and ACLs

Give each account only the folders it needs:

```sh
chown -R deploy:web /srv/app
chmod -R u=rwX,g=rX,o= /srv/app
setfacl -R -m u:webro:rX /srv/app /var/log/app
setfacl -R -d -m u:webro:rX /srv/app /var/log/app   # new files inherit the ACL
```

- Keep secrets (`.env`, keys) readable only by the service account, not by `webro`.
- An `only` folder rule in Werbot should match these permissions: the rule explains the intent and audits it, the permissions enforce it.

## Accounts for files only

An account that only transfers files needs no shell. In `sshd_config`:

```text
Match User files
    ChrootDirectory /srv/files
    ForceCommand internal-sftp
    AllowTcpForwarding no
    X11Forwarding no
    PermitTTY no
```

The chroot folder and every folder above it must be owned by `root` and not writable by group or others; give the account a writable folder inside it.

## Narrow sudo

Prefer separate accounts to `sudo`. Where `sudo` is needed, allow exact commands with full paths and fixed arguments:

```text
deploy ALL=(root) NOPASSWD: /usr/bin/systemctl restart web.service, /usr/bin/systemctl status web.service
```

- No `ALL`, no wildcards in commands or arguments.
- No editors, pagers, shells, interpreters or file tools (`vi`, `less`, `bash`, `python`, `tee`, `cp`) as allowed commands: they can run other programs or write any file.
- Use `sudoedit` for editing a fixed file instead of an editor under `sudo`.
- Keep `Defaults use_pty` and a log of `sudo` on the host.

## Login shell and startup files

The command console runs each line through the login shell of the target account, which reads its startup files ([command console](policies.md#command-console)).

- Give the account a POSIX shell (`sh`, `bash`, `dash`, `zsh` or `ksh`).
- Make `~/.profile`, `~/.bashrc`, `~/.bash_profile` and `~/.zshrc` owned by `root` and not writable by the account, so functions and aliases there cannot change what a line runs.

## Checklist

- [ ] One account per access level, no shared account, no `root` logins.
- [ ] Folder permissions and ACLs match the Werbot folder rules.
- [ ] File-only accounts use `ChrootDirectory` and `ForceCommand internal-sftp`.
- [ ] `sudoers` lists exact commands only.
- [ ] Startup files of the login shell are not writable by the account.
- [ ] Werbot rules on top: folder rules, the allowed-commands preset and a `console` terminal rule where people work by hand.
