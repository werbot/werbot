# Phase 2 parity

Every parity item of the phase 2 design has a test or a Playwright spec that proves it.
`go test ./test/parity/` checks that each name below exists, that each linked doc anchor exists, and that the tables have 25 and 9 rows.

Red-team tests (`test/redteam`), cluster scenarios (`test/cluster`) and Playwright specs run on a stand. The other tests run in `make test`.

## Parity

| # | Item | Proof |
| --- | --- | --- |
| 1 | Role and resource request with a reason | `TestCreateRequestReasonAndDuration`, `web/e2e/requests.spec.ts`; [access requests](../jit.md#access-requests) |
| 2 | Approval rules | `TestSelfApprovalForbidden`, `TestFourEyesExcludesSponsor`; [approval policies](../jit.md#approval-policies) |
| 3 | Approval timeout is a denial | `TestRedteamApprovalTimeoutDenies`, `TestRedteamApprovalDecidedAfterTimeout` |
| 4 | Duration ceilings | `TestGrantDuration`, `TestExpire`; [organization settings](../jit.md#organization-settings) |
| 5 | Auto-approval, manual approval wins | `TestAutoBlockedByManual` |
| 6 | Session ends on grant expiry and revoke | `TestRedteamGrantExpiryBreaksSession`, `TestRedteamGrantRevokeBreaksSession`, `TestScenario18GrantExpiresWhileReplicaIsKilled`; [ending sessions](../jit.md#ending-sessions) |
| 7 | Break-glass with recording | `TestBreakGlass`, `TestE2EBreakGlassIsRecorded`; [break-glass](../jit.md#break-glass) |
| 8 | Per-session MFA, WebAuthn, window, session binding | `TestE2EChallengeOfOtherUserRejected`, `TestE2EMFAReuseWithinWindow`, `web/redteam/sso.redteam.ts` |
| 9 | MFA with plain `ssh` | `TestE2ESSHMFAApprovePage`, `TestE2ESSHMFATOTP`, `TestE2EOpenSSHClientMFA`; [SSH](../mfa.md#ssh) |
| 10 | MFA on admin actions | `TestRequireFreshMFA`, `web/redteam/sso.redteam.ts`; [fresh MFA](../mfa.md#fresh-mfa-for-admin-actions) |
| 11 | OIDC with claims mapping and PKCE | `TestApplyMapping`, `web/redteam/sso.redteam.ts`; [claims mapping](../sso.md#claims-mapping) |
| 12 | CLI login: loopback and device flow | `TestLoginLoopback`, `TestLoginDevice`, `TestScenario20CLILoginThroughSeparateWeb`; [log in](../cli.md#log-in) |
| 13 | CLI session TTL in settings | `TestRefreshSlidingTTL`, `TestRefreshAfterMaxLowered` |
| 14 | Live view | `TestScenario15LiveViewAcrossReplicas`, `TestE2ELiveObserve`; [live view](../web-terminal.md#live-view) |
| 15 | Join as observer | `TestObserveIgnoresInput`, `TestLiveDropsBrowserInput` |
| 16 | Terminate in 5 s or less | `TestScenario16TerminateFromOtherReplica` (2 s or less); [terminate](../web-terminal.md#terminate) |
| 17 | Locks | `TestRedteamKillSwitchTargets`, `TestStrictPolicyStale`; [locks](../jit.md#locks) |
| 18 | Recording fails closed | `TestSpoolCapTerminatesRecorder` |
| 19 | Web terminal with recording | `TestScenario15WebTerminalAcrossReplicas`, `web/e2e/terminal.spec.ts` |
| 20 | Re-evaluation in 5 s or less | `TestRedteamGrantRevokeBreaksSession` (2 s or less), `TestRedteamFrozenAgentLosesTokenAndSession` |
| 21 | Obligations | `TestApprovalObligation` and `TestRedteamApprovalGrantedRunsCommand` (`approval`), `TestRedteamSecretNotVisibleToAgent` (`mask`), `TestE2ESSHMFAApprovePage` (`mfa`), `TestE2EExecDeniedWithMessage` (`@message`); [command approvals](../policies.md#command-approvals), [output masking](../policies.md#output-masking) |
| 22 | MCP RBAC per tool | `TestAgentBaselineReadOnlyExactSet`, `TestRedteamPromptInjection`; [Werbot MCP](../agents.md) |
| 23 | MCP audit with arguments | `TestRedteamDenialHasAuditTrail`; [run timeline](../agents.md#run-timeline) |
| 24 | MCP `ssh_exec` with timeout and output limit | `TestSSHExecTimeout`, `TestSSHExecTruncates`; [limits](../agents.md#limits) |
| 25 | Agent kill switch | `TestRedteamFrozenAgentLosesTokenAndSession`; [kill switch](../agents.md#kill-switch) |

## Failure matrix

The nine rows of the phase 2 failure matrix. The user-facing table is in [failure modes](../failure-modes.md).

| Failure | Test |
| --- | --- |
| Server unavailable while a request waits for approval | `TestRedteamApprovalDecidedAfterTimeout`, `TestApprovalServerDownTimesOut` |
| Replica dies under a web terminal or live view | `TestScenario15KillBridgeReplica` |
| `NOTIFY` lost | `TestResendDecisionsOnConnect`, `TestTerminateResentOnResync` |
| Internal listener unreachable | `TestScenario15RelayUnavailableWhenInternalListenerDown` |
| Authorization server unavailable | `TestSignerKeepsValidCertWhenRenewFails` |
| Client clock skew | `TestSignerClockSkew` |
| `lock_mode = strict` without a tunnel | `TestStrictPolicyStale` |
| Identity provider unavailable | `TestOIDCIdPDownRefusesLogin`, `TestRecheckKeepsSessionsWhenIdPDown` |
| MFA confirmation did not arrive | `TestMFATimeout`, `TestE2EMFATimeout` |
