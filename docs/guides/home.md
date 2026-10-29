# The home page

The home page (`/`) answers two questions at a glance: is everything OK, and what needs me? The numbers update every 15 seconds while the tab is visible, and stop while it is in the background.

## What it shows

- **A banner when jobs are stuck.** When queued jobs have no active runner with the labels and isolation level they ask for, a banner says how many and how to fix it. Owners and admins get **Add a runner**, which opens Registration tokens (`/runners/tokens`); everyone else gets **Show the waiting jobs**, which opens `/jobs?state=queued`. See [Follow jobs](jobs.md) for how matching works.
- **Four numbers**, each a link to the matching page:
  - **Running now** (blue): jobs in the `leased` or `running` state. Opens `/jobs?state=running`.
  - **Queued** (violet): jobs in the `queued` state, with a note when some have no matching runner. Opens `/jobs?state=queued`.
  - **Failed in the last 24 hours** (coral): jobs that ended `failed` or `timed_out` in the last 24 hours. Opens `/jobs?state=failed,timed_out`, which lists both states.
  - **Runners connected** (green): active runners seen in the last two minutes, with a note when some are offline. Opens `/runners`. Viewers cannot read runners, so they see three numbers.
- **Needs your attention:** stuck jobs, server checks that fail, offline runners and pending invitations. Only the rows that exist appear, each with its count and a link. When nothing needs you, it says so in one line.
- **Latest jobs** (the five newest), **Runners** (the first five), **Latest events** (the five newest audit events, for owners and admins only) and **Server status** (the same summary as `/system`, with each check and the version).

Each card shows a skeleton while it loads, an error with **Try again** if it cannot load, and a plain sentence when it has nothing yet.

The side menu shows the same counts next to Jobs, Runners and Invitations. See [UI principles](../ui-principles.md#2-navigation-model-adr-0022-adr-0059).

## The API behind it

The numbers and the counters come from one request:

```http
GET /api/v1/organizations/{id}/overview
```

```json
{
  "Success": true,
  "Message": "overview.loaded",
  "Data": {
    "Jobs": { "Running": 1, "Queued": 4, "QueuedNoRunner": 3, "FailedLast24h": 2 },
    "Runners": { "Online": 1, "Offline": 2, "Paused": 0 },
    "Invitations": { "Pending": 2 }
  }
}
```

- **Who may call it:** any member of the organization (`org.read`), with a session or an API token. A caller who is not a member gets 404.
- **Each block follows the permission of its list.** `Jobs` needs `job.read`, `Runners` needs `runner.read` and `Invitations` needs `member.manage`. A block the caller may not read is `null`, never zero, so a client can tell "none" from "not yours to see". An API token also needs the scope for each block.

| Role | `Jobs` | `Runners` | `Invitations` |
|---|---|---|---|
| Owner, admin | ✓ | ✓ | ✓ |
| Maintainer, developer | ✓ | ✓ | `null` |
| Viewer | ✓ | `null` | `null` |

- **What each count means:**
  - `Jobs.QueuedNoRunner` counts the queued jobs whose `QueuedReason` is `NO_MATCHING_RUNNER` in the job list. It is part of `Jobs.Queued`.
  - `Jobs.FailedLast24h` counts `failed` and `timed_out` jobs that finished less than 24 hours ago. `canceled` and `lost` are not counted.
  - `Runners.Online` and `Runners.Offline` use the `Status` rule of the runner list: an active runner is online when it was seen in the last two minutes. `Runners.Paused` counts disabled runners. Revoked runners are not counted.
  - `Invitations.Pending` counts invitations that are not accepted, revoked or expired.
- **Cost:** three fixed queries, whatever the size of the organization. The job count reads only active jobs and jobs that failed in the last day, through two partial indexes, so it stays small as history grows.

The full contract is in [`packages/openapi/openapi.yaml`](../../packages/openapi/openapi.yaml) (operation `getOrganizationOverview`).
