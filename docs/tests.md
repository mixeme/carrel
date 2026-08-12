# Tests

What is tested, how it is tested, and what is deliberately left to a person. About 420 test functions across 20 packages.

Running them is in [development.md](development.md); the acceptance criteria they answer to are §21 of [carrel-spec.md](carrel-spec.md).

---

## The three tiers

| Tier | Command | Needs | Is it a gate? |
|---|---|---|---|
| Unit and handler | `go test ./...` | nothing | **Yes.** Must be green before a commit |
| Race | `CGO_ENABLED=1 go test -race ./...` | a C compiler | Yes where one is available |
| Integration | `go test -tags=integration ./...` | live Baikal and WebDAV | Advisory; skips silently without credentials |
| Manual | a person, a phone, jtx Board | real clients | [manual-acceptance.md](manual-acceptance.md) |

`go test ./...` needs no network and no server. Everything that speaks DAV runs against a fake server in `httptest`, which is what makes the suite fast enough to run on every change and honest enough to be worth running.

---

## What each package proves

### `crypto` — the parts that cannot be checked by looking

Derivation with separated salts, so the login verifier cannot be used to attack the KEK. Wrap and unwrap round trips. A password change that leaves data readable and a reset that does not. Tampered ciphertext and a wrong password failing identically. The AAD constants actually separating their domains — a wrapped DEK offered as sealed state is refused. The server key persisting, and a corrupt one being reported rather than replaced. Escrow round trips, isolation between two escrows, master password rotation leaving deposits intact.

Invite tokens get their own file: full entropy, a digest that is deterministic and collision-free, and comparison only through the constant-time path — including against a digest differing in one bit, a truncated one, and none at all.

### `store` — the file on the volume

State encrypted and reloaded. An atomic write leaving no stray file behind. A failed write leaving neither record nor audit entry. Invite indistinguishability across unknown, revoked, expired and spent. The last administrator surviving deletion, demotion and disabling. A DEK preserved across a user's own password change and replaced by an administrator's reset. Escrow not applying retroactively. Audit bounded, and containing no secret of any kind.

### `session` — expiry and the cache

Idle and absolute expiry, rotation, per-user teardown, and keys wiped on every path that ends a session. For the cache: hit, miss, LRU eviction, and wipe. A body cached at an older ETag being a **miss**, which is the property that makes a stale read impossible rather than unlikely. Thumbnails evicting under their own stricter ceiling.

### `dav` — the wire

Multistatus parsing. The SSRF guard refusing loopback, private and link-local addresses, refusing them again after a redirect, and honouring an allowlist. `GET` returning a stream that can be read in pieces. Request bodies **asserted**: a `PROPFIND` or a `REPORT` has to name the properties it wants under their own namespaces, and it is asserted because getting this wrong once already cost `addressbook-multiget` silently returning nothing.

### `dav/discovery`

The chain against a mock server: well-known, principal, home-sets, collections. A calendar collection decoded from a real-shaped answer. File collections found as the plain collections under the root, with the containers holding the DAV homes excluded — Baikal answers its root with `calendars/` and `addressbooks/`, and mistaking those for folders of files is the failure this prevents. A server with neither calendars nor address books connecting anyway, because a separate account for files is a supported arrangement.

### `model` — the promise not to lose data

The largest set, because §8 is the promise that is hardest to keep and easiest to break by accident.

A contact with X- properties keeping every original property after its name is edited, compared property by property. A vCard 3.0 still being 3.0 after its photo is replaced. A photo held as a link shown but not offered for editing. An object written and read back serialising identically, so a difference after a `PUT` is the server's doing and not the encoder's — with the two rewrites a round trip does make (property names upper-cased, a `TYPE` list becoming repeated parameters) named and pinned rather than left to be discovered.

`ATTACH` specifically: parsed with its parameters across a folded line; an inline base64 attachment marked and never rewritten as a link; the property modelled rather than listed among foreign ones while `X-JTX-COLOR` still is; a detach rewriting the set, preserving the remaining inline attachment's parameters and an unrelated property; the last one being removed rather than emptied; a round trip reporting no loss; and the Markdown links surviving export and import.

Loss detection: a dropped X- property found, fewer instances reported as reduced, a changed value reported as changed, parameter order ignored, server-added and volatile properties ignored, and the registry reporting the first novel loss while treating a repeated one as a trait of that server.

### `merge` — the scoring table

Each row of §15 as a case: one strong signal reaching the threshold, name and birthday together not, different kinds never scoring, a record with nothing to score on matching nothing. Phone normalisation including `tel:` URIs and short codes; name normalisation with words sorted, so «Ivan Petrov» and «Petrov, Ivan» are one person. Partial birthdays. Clustering through shared signals, and the rejected pair that is never grouped. The threshold changing what is offered. Field merge with a conflict and with a stale preference. A merged patch that adds without overwriting.

### `fanout` — the concurrency

Merge order, per-source and overall timeouts, retry, cancellation keeping partial results, a cancelled task opening no further connections, and concurrent snapshots against retries and subscribers. This last is the goroutine-leak and race gate of §21 and is the main reason `-race` matters.

### `provider/contacts`, `provider/calendar`

Against a fake address book and calendar over HTTP: reopening an unchanged collection making **no requests at all**; an unchanged collection tag sparing the deep `PROPFIND` after the TTL; multiget batching and preserving the order asked for; a body cached at an older version being a miss; a malformed record costing only itself; a second create at the same path refused rather than overwriting; an edit made elsewhere first producing a conflict with both versions and no write; a dropped X- property reported once and aggregated after. For the calendar: `calendar-query` and component queries returning only what was asked for even when the server answers with everything, weekly RRULE expansion in range, and a 412 becoming a conflict.

### `provider/files`

Path resolution in both directions — traversal refused, a name that only looks like one accepted (`..hidden`, `a..b/c`), backslash and control bytes refused, and `Relative` refusing a path outside the collection, which is the check that keeps a foreign `ATTACH` from naming somebody else's tree.

Listing separating folders from files without reaching into a subfolder, and reading size, type and modification time. A megabyte read out of a stream in two goes, which is what shows the body was not buffered. A range returning the middle of a file. A refused conditional create leaving the original intact. A taken name yielding the next one without touching what was there. `EnsureDir` creating a chain once and being content the second time. A folder listing cached inside the TTL at no request, and a write making it a miss. The listing ceiling truncating and saying so.

### `web/handler` — over the real middleware chain

These are the tests that catch what unit tests cannot, because they run through the actual chain: session, body limit, security headers, CSRF, the guards, routing and rendering.

**`acceptance_test.go`** walks §21 end to end: an empty volume, the first administrator, an invitation handed over by link alone with SMTP unset, the account it creates, and that account signing in with the password it chose. Along the way: no credentials and no data key existing for an invited login until it is accepted, a spent link answering gone with only its digest ever stored, and the new account refused at the panel. Plus an administrator's key not opening another account's data, disabling an account ending its live sessions and wiping their keys then and there, the last administrator surviving everything, and the destructive reset naming escrow as the alternative above the button that does it.

**Per stage:** `stage5_test.go`, `stage6_test.go` and `stage7_test.go` each hold that stage's behaviour against a fake server. The rest cover authentication, CSRF, the admin panel, escrow, invitations, contacts, the calendar and the shell.

Stage 7 in particular: the navigation entry appearing only when a plain collection exists; the listing and breadcrumb; download headers and body; a traversal in a download URL answering 404; an upload refusing to overwrite; a read-only collection refusing writes and offering no forms; mkdir and delete; the whole of §23.10 in one pass — folder named once, file stored under a date-and-title name, `ATTACH` written as a URI with a filename and not as base64, the card showing it with its proxy; the proxy serving the bytes; a foreign link shown but refused; an inline attachment refused with a reason and no write; detaching leaving the file and saying so; an unreachable file server still rendering the card; a card with no folder configured pointing at the setting; and a folder of Markdown importing from WebDAV without disturbing it.

### `mail`, `photo`, `ratelimit`, `config`, `account`

SMTP against a local relay that can be told to refuse: a missing configuration reported without dialling, an unreachable relay naming the address it tried, a refusal passed through in the server's own words — and the relay password staying out of the transcript shown on screen. Photo processing: EXIF orientation applied, metadata gone after re-encoding, a pixel ceiling refused before decode. Rate limiter progression and key independence. Configuration defaults, file, environment precedence, and a bad value refused. The sealed blob round-tripping with its selections and decisions.

---

## Acceptance criteria and the tests that answer them

§21 of the specification is a list of things that must be true. Most of them are asserted somewhere, and a criterion whose test is named here is a criterion that cannot regress quietly. This table is worth keeping current: it is the difference between "we have tests" and "this requirement is covered by that test".

| §21 criterion | Test |
|---|---|
| An administrator sees no other user's connections or data | `TestAdminCannotReadAnotherAccountsData` |
| An invited user chooses their own password; no hash or key exists before they accept | `TestStageOneAcceptanceFlow` |
| An invite link works with SMTP entirely unconfigured | `TestInviteWorksWithoutSMTP` |
| An invite token is single-use and only its digest is stored | `TestInviteStoresOnlyDigest`, `TestHashToken` |
| A test email shows diagnostics when SMTP is wrong | `TestSendReportsAnUnreachableRelay`, `TestSendPassesTheServersRefusalThrough` |
| An administrator's reset is announced as destructive before it happens | `TestResetPasswordIsAnnouncedAsDestructive`, `TestResetPasswordReplacesDEK` |
| A user's own password change keeps every connection | `TestChangePasswordKeepsDEK`, `TestProfilePasswordChange` |
| Disabling a user ends their active sessions immediately | `TestDisableEndsActiveSessionsAtOnce` |
| The last administrator cannot be deleted, demoted or disabled | `TestLastAdminSurvivesThePanel`, `TestLastAdminGuard` |
| The login page offers no misleading password recovery | `TestForgotOffersNoReset` |
| With escrow off, no copy of any key is available to anyone else | `TestEscrowOffByDefault` |
| Enabling escrow grants nothing over accounts that predate it | `TestEscrowAppliesOnlyToLaterUsers`, `TestRecoveryRefusedWithoutADeposit` |
| Recovery is impossible without the master password | `TestRecoveryNeedsTheMasterPassword`, `TestRecoveryIsThrottled` |
| Every recovery is in the audit log and in a mail to the user | `TestRecoveryThroughThePanel`, `TestEscrowActionsAreAudited` |
| The profile says whether escrow covers this account | `TestEscrowCoversNewAccountsAndSaysSo`, `TestForbiddenOptOutIsVisible` |
| `/about` is reachable without logging in | `TestAboutPublic`, `TestAboutNoSessionRequired` |
| An X- property survives an edit to the name, compared byte for byte | `TestApplyKeepsForeignProperties`, `TestMarshalPreservesEveryProperty` |
| A vCard 3.0 is still 3.0 after its photo is replaced | in `model/photo_test.go` |
| An edit from a second client produces a choice, not an overwrite | `TestConflictScreenOn412` |
| Reopening an unchanged collection makes no new multiget requests | in `provider/contacts/contacts_test.go` |
| An outside edit becomes visible after a refresh rather than serving the cache | in `session/cache_test.go`, `provider/contacts/contacts_test.go` |
| Signing out frees the cache | in `session/cache_test.go` |
| Records from a slow source land in the right sort position | in `fanout/fanout_test.go` |
| A failed source is marked with a retry of its own; the rest stay | in `fanout/fanout_test.go` |
| Leaving a page mid-poll leaves no goroutines | in `fanout/registry_test.go` (needs `-race`) |
| Unticking a collection persists across a restart | in `store/accounts_test.go`, `handler/stage5_test.go` |
| One person in two books of two accounts is a candidate | in `merge/merge_test.go` |
| "Not duplicates" survives a restart | in `store/duplicates_test.go` |
| A server merge whose `PUT` fails deletes no source | in `handler/stage6_test.go` |
| Deleting a linked record from another client causes no error | in `handler/stage6_test.go` |
| A 10 MB download does not exhaust memory | `TestLiveLargeFileStreamsWithoutBuffering` (integration), `TestOpenStreamsWithoutBuffering` |
| Removing an `ATTACH` leaves the file on the server | `TestDetachLeavesTheFileOnTheServer` |

The criteria not in this table are the ones a person has to check, below.

---

## How the fakes work

Three helpers, worth reusing rather than rebuilding.

**A fake DAV server** is an `httptest.Server` holding a map of path to body, answering `PROPFIND` and `REPORT` with a hand-built multistatus and `GET`/`PUT`/`DELETE` against the map. It counts requests, which is how "reopening an unchanged collection makes no requests" is asserted rather than assumed, and it can be told to fail a specific write, which is how conflict and partial-failure paths are reached. `stage6_test.go` (`dupBooks`) and `stage7_test.go` (`davHost`) are the two to copy from; the second serves a calendar and two file collections from one server, which is the arrangement attachments need.

**`newApp`** in `auth_test.go` wires a whole service against a temporary directory, with Argon2id turned right down — the cost profiles are the crypto package's business, and at production settings a login per case would dominate the run. It keeps cookies between requests, so `a.get` and `a.post` behave like a browser, and `a.post` fills in the CSRF token unless the test is specifically about not having one.

**`a.postMultipart`** submits an upload. It has a variant that omits the `X-CSRF-Token` header, which matters more than it looks: a plain HTML form cannot set a header, so its token is a field, and the CSRF check then has to read the multipart body to find it. That is the path that was silently answering 403 for any upload over a megabyte, and the test that omits the header is what keeps it fixed.

---

## What is deliberately not automated

Some things cannot be tested without the thing they are about. These are in [manual-acceptance.md](manual-acceptance.md), and five of them block a v1 release:

| | Why a person has to do it |
|---|---|
| A note round-tripping through **jtx Board** | The compatibility claim of §23.9 is about a real client's behaviour, and a fake one that agrees with us proves nothing |
| The same through **Evolution** | As above, on the desktop |
| `docker compose` from an empty volume through a full scenario | Read-only filesystem, dropped capabilities, non-root, health check — the container is the unit and it has to be run |
| Debug logs holding no password or token after a full scenario | A test can assert about a line it thought to look at; a person reads what is actually there |
| **Pasting a screenshot into a note taking a couple of seconds** | §23.10 says this outright: if attaching is slow, the feature is dead, and no assertion measures whether something felt instant |

Beyond those: real-world duplicate detection on a genuine address book (false positives are a judgement, not an assertion), print output on paper, EXIF orientation on a photo from an actual phone, the progress stream through a real Apache, a 500-contact book feeling acceptable, and `ATTACH` links opening in third-party clients — which §23.10 already says will not always work and names as a property of the approach.

---

## Adding a test

Put it where the behaviour lives. A property that must survive an edit belongs in `model`; a request that must or must not be made belongs in the provider, where requests can be counted; a status code, a header or a rendered string belongs in `handler`, over the real chain.

Two things worth doing that are easy to skip:

**Assert the request, not only the answer.** The `PROPFIND` bug that made every request name properties the server did not recognise passed every test that only looked at parsed responses. If the wire format matters, assert the bytes.

**Name what the test is protecting.** A test called `TestUploadCreateIsConditional` with a comment saying a create is conditional so an upload never silently replaces somebody's file is a test the next person will not delete when it becomes inconvenient. One called `TestUpload2` is.
