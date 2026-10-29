# Follow jobs in the web app

Open **Run › Jobs** in the side menu (or type "jobs" in the command palette). The list lives at `/jobs` and each job has its own page at `/jobs/<id>`, so you can bookmark or share a link to it. Every member of the organization sees the list. Viewers see the jobs but not their logs.

## The list

- Newest first, 20 to a page, with each job's state, when it was created, how long it waited, ran or took, its attempts and, for a job that ended badly, the reason in plain words.
- The chips above the list filter by state. The filter is part of the address (`/jobs?state=queued`), so the numbers on the [home page](home.md) open the list already filtered.
- **Jobs that no runner can take are called out at the top.** A queued job needs an active runner in the organization that has every label the job asks for and supports its isolation level. When none exists, the list shows how many jobs are stuck, marks each one, and offers to show only the waiting jobs. Register a runner with the right labels ([Register a runner](../runner/register.md)) and they start on their own. A job still waiting after 24 hours fails with `QUEUE_TIMEOUT`.
- The list refreshes every 10 seconds while any job on the page is still active.

## A job

- **State and times:** queued, started, finished, duration, the deadlines, and the reason when it failed, timed out, was lost or canceled.
- **Stop the job:** owners, admins and maintainers can stop a job that has not finished, after a confirmation. A queued job stops at once. A running one stops when its runner sees the request, usually within 10 seconds.
- **Job log:** the log of the latest attempt, or of any earlier one. A running attempt streams live over server-sent events, and the view follows the newest line until you scroll up; "Jump to the latest line" brings it back. If the connection drops, the browser reconnects and continues after the last line it received. Colors from ANSI escape codes are shown; other terminal control codes are removed. The view keeps the last 5,000 lines. Developers and above can read logs.
- **Attempts:** each attempt with its state, reason, exit code, the runner that took it and its fencing token. A newer attempt always has a larger token, and the server refuses any call that carries an older one.
- **Settings:** the image, the steps (a step marked "Changes a target" stops automatic retries once it begins), the resources, the time limit, the labels, the isolation level and the hosts the job may reach. Secret values never appear.

## Create a job through the API

Pipelines (milestone M4) will create jobs on their own. Until then, an owner or admin creates one through the API. When the organization has no jobs yet, the empty list links here.

```sh
curl -X POST https://dhow.example.com/api/v1/organizations/<org-id>/jobs \
  -H 'Content-Type: application/json' --cookie ... --header 'X-CSRF-Token: ...' \
  -d '{
    "Name": "build-web",
    "Spec": {
      "Image": "node:22@sha256:<digest>",
      "Steps": [{"Name": "test", "Run": "npm ci && npm test"}],
      "Labels": ["linux-builders"],
      "Isolation": "container"
    }
  }'
```

- `<org-id>` is `Memberships[].OrganizationId` in the answer of `GET /api/v1/me`. The request needs the session of an owner or admin, with a second factor when the organization asks for one.
- The image must be pinned by digest. The full spec, including resources, secrets and allowed hosts, is in [the API reference](../../packages/openapi/openapi.yaml) under `CreateJobRequest`.
- The job waits in the queue until an active runner has every label in `Labels` and supports its `Isolation`.
