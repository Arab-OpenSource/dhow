# خطة M3: جهاز التشغيل والعزل (Runner & isolation)

**الحالة:** القرارات اتاخدت بتفويض من Sabry ([ADR-0041](ADR/0041-runner-identity-and-registration.md) لحد [ADR-0048](ADR/0048-runner-packaging-and-kubernetes.md))، ومستنية مراجعته قبل التنفيذ · **التاريخ:** 2026-10-27

## الهدف
شرط خلاص M3 في `docs/roadmap.md`:
- **job بيشتغل معزول:** الـ escape suite والـ limits suite خضرا على الـ driver الافتراضي (Podman rootless)، وعلى Kubernetes.
- **قتل الـ runner في نص الـ job بيترتّب صح:**
  - الـ attempt بيبقى `lost`، والـ job بيتعاد مرة واحدة بس لو مفيش خطوة mutating بدأت، أو بيقفل `lost` لو بدأت.
  - مفيش اتنين attempts شغّالين في نفس الوقت أبدًا.
  - مفيش حاوية فاضلة.

المراجع:
- [ADR-0003](ADR/0003-go-for-server-and-runner.md) و[ADR-0005](ADR/0005-api-rest-openapi-and-connect.md) و[ADR-0006](ADR/0006-postgres-default-redis-optional.md) و[ADR-0007](ADR/0007-job-isolation.md) و[ADR-0014](ADR/0014-secrets-management.md) و[ADR-0030](ADR/0030-server-modular-monolith.md) و[ADR-0035](ADR/0035-identity-core-implementation.md) و[ADR-0036](ADR/0036-interactive-host-installer.md).
- `docs/architecture.md` (الأقسام 2.2 و4 و5 و6) و`docs/security-model.md` (الأقسام 2 و7 و8) و`docs/pipeline-spec.md`.

## النطاق
1. binary اسمه `dhow-runner`، وتسجيله، وهويته (مفاتيح Ed25519 وX-Wing، وتوكن قصير، وإلغاء فوري).
2. الـ runner plane في السيرفر: Connect على `/runner-api/`، وleases بـ fencing، وheartbeat، وإنهاء idempotent.
3. sandbox بـ Podman rootless: حدود cgroup v2، وrootfs للقراية بس، ومن غير capabilities، وseccomp بتاعنا، وagent هو PID 1.
4. شبكة `none`، والـ egress من خلال proxy في الـ runner بقايمة hosts.
5. gVisor كمستوى عزل اختياري.
6. نقل الأسرار بـ HPKE ونقاط الـ masking (التخزين نفسه في M5).
7. الـ logs: chunks متسلسلة، وتخزين في Postgres، وSSE بيكمّل من آخر seq، وسقف واحتفاظ.
8. الإلغاء بالتدريج، والـ reconciliation، والحالات بقيود في قاعدة البيانات.
9. التغليف: systemd، ووضع `runner` في الـ installer، وHelm chart للسيرفر والـ runners، وpod لكل job على Kubernetes.
10. واجهة صغيرة: أجهزة التشغيل، وتوكنات التسجيل، والـ jobs ومعاها log حي.

## مش في M3
- `.dhow.yml` والـ DAG والـ matrix والـ CEL والـ artifacts والـ cache، وGit providers: كلها M4. الـ job في M3 بيتعمل من `POST /jobs` بس ([ADR-0047](ADR/0047-job-state-machine-and-reconciliation.md)).
- تخزين الأسرار وتشفير الـ envelope والـ KEK والموافقات: M5. M3 بيعرّف `SecretResolver` والنقل والـ masking.
- الـ deploy targets والعلامة `mutating` الحقيقية: M6. M3 بيبني الحاجز (`BeginStep`) ويختبره بخطوات متعلّمة في الاختبار.
- الـ log viewer الكامل (virtualized): M7.
- Firecracker microVM: M8 (`ee/`). الـ driver interface جاهز له.
- الـ object storage (S3 والـ disk) والـ log segments: M4.
- أكتر من replica للسيرفر (HA): Business، وبعد الـ blob store.
- runners على Windows وmacOS.
- runners مشتركة بين المؤسسات.
- autoscaler.
- بناء images جوه الـ job (user namespaces متداخلة).
- registries خاصة بـ credentials.
- egress شفاف لبرامج مابتعرفش proxy.

## المعمارية
```
                         ┌──────────────────────────── dhow-server ─────────────────────────────┐
 browser ──HTTPS──► proxy ──► /api/v1  REST + SSE (session cookie, CSRF, can())                   │
                         │        Modules/Runners   runners, registration tokens                  │
                         │        Modules/Jobs      jobs, attempts, leases, logs, reconciler ─────┼──► PostgreSQL 18
 dhow-runner ─HTTPS─► proxy ──► /runner-api  Connect, unary only                                  │    state, SKIP LOCKED,
   (outbound only)       │        Core/RunnerAuth   WithRequestGate: bearer + runner state         │    LISTEN/NOTIFY,
                         │        signs LeaseGrant (Ed25519), seals secrets (HPKE X-Wing)         │    LogChunks by day
                         └───────────────────────────────────────────────────────────────────────┘

 runner host  (dhow-runner.service: User=dhow-runner, Delegate=yes, MemorySwapMax=0)
   Executor ── Lease ─ Start ─ Heartbeat (10s) ─ BeginStep/EndStep ─ Complete      Journal (IDs + fencing only)
     │   self-fence: last heartbeat sent + 45s - 10s ──► kill sandbox
   Masker ─► Chunker (64 KiB / 250 ms, seq) ─► memory queue ─► spill file ─► LogService.Append
   Egress proxy (one per attempt, allowlist, deny private ranges) ◄── egress.sock ──┐
   podman rootless (crun | runsc, our containers.conf, inspect-after-create)        │
     └─► container: --network=none --read-only --cap-drop=ALL no-new-privileges     │
           userns=auto, seccomp (ours), cgroup: cpu/memory/pids, /workspace tmpfs   │
           PID 1 = agent ◄── control.sock ──► Executor   (agent exits → container dies)
           127.0.0.1:3128 ───────────────────────────────────────────────────────────┘
           steps: /bin/sh -e -s  (script on stdin, env + secrets for the step only)

 Kubernetes  (driver "kubernetes")
   runner StatefulSet (restricted) ── creates ──► Pod per attempt in ns dhow-jobs (restricted, hostUsers:false,
                                                  optional runtimeClassName gvisor, dedicated node pool)
   NetworkPolicy: job pods ─► runner pods :7443 (control) :3128 (egress) only
```

## القرارات
| # | القرار | ليه في سطر | ADR |
|---|---|---|---|
| 1 | هوية الـ runner: مفاتيح Ed25519 (هوية) وX-Wing (أسرار) بتتولّد عنده وبتتثبّت في السيرفر، وتوكن تسجيل لمرة واحدة محصور وبينتهي، وaccess token بيعيش 15 دقيقة من challenge موقّع، وتوقيع جديد عند `Start`. مش mTLS | الـ TLS بيتقفل عند الـ reverse proxy، وCA داخلي كان هيبقى سر من الدرجة الأولى ومحتاج CRL | [0041](ADR/0041-runner-identity-and-registration.md) |
| 2 | الـ runner بيتأكد من السيرفر بالـ TLS وبتوقيع Ed25519 على كل `LeaseGrant` بمفتاح متثبّت وقت التسجيل | proxy أو ingress مخترق مايقدرش يحقن jobs | [0041](ADR/0041-runner-identity-and-registration.md) |
| 3 | Connect بـ RPCs unary بس على `/runner-api/`، وlong-poll لحد 25 ثانية، وlistener منفصل اختياري بـ TLS | بيعدّي من أي proxy على HTTP/1.1 أو HTTP/2 من غير إعداد خاص | [0042](ADR/0042-runner-protocol-and-leasing.md) |
| 4 | الـ lease مدته 45 ثانية وheartbeat كل 10، وfencing token من sequence واحدة، وتسليم at-least-once بإنهاء idempotent، والـ runner بيقفل نفسه قبل ما السيرفر يعتبر الـ lease ضاع | مفيش attempt بيتداخل مع attempt تاني حتى مع انقطاع الشبكة | [0042](ADR/0042-runner-protocol-and-leasing.md) |
| 5 | Podman 5.4+ rootless بالـ CLI، وcrun، وإعدادات Podman بتاعتنا، والتأكد بـ inspect بعد الإنشاء | image handling مجرّب ومن غير daemon ولا socket، والإعدادات الفعلية بتتأكد مش بتتفترض | [0043](ADR/0043-job-sandbox-rootless-podman.md) |
| 6 | الـ agent (الـ binary نفسه) هو PID 1، والخطوات على unix socket، والأسرار في env الخطوة بس | الأسرار مابتتكتبش في إعدادات Podman، وموت الـ runner = موت الحاوية | [0043](ADR/0043-job-sandbox-rootless-podman.md) |
| 7 | حدود cgroup v2 صارمة (CPU وذاكرة وPIDs وtmpfs للقرص ووقت)، والـ runner مابيقومش من غير التفويض | fail closed بدل حدود على الورق | [0043](ADR/0043-job-sandbox-rootless-podman.md) |
| 8 | gVisor بـ `--runtime=runsc` بعد spike ليه شروط قبول، والبديل Kubernetes RuntimeClass. وmicroVM بعدين driver تالت | gVisor مع Podman rootless أقل تجربة، فالقرار مربوط بنتيجة | [0043](ADR/0043-job-sandbox-rootless-podman.md) |
| 9 | الشبكة `none` دايمًا، والـ egress بـ forwarder في الـ agent على unix socket لـ proxy في الـ runner بقايمة hosts، ومفيش DNS، والنطاقات الخاصة ممنوعة | مفيش اعتمادية شبكة على الـ host، والفلترة بالاسم بتشتغل مع الـ CDNs | [0044](ADR/0044-job-network-egress-proxy.md) |
| 10 | الأسرار مشفّرة بـ HPKE (X-Wing وAES-256-GCM) لمفتاح الـ runner ومربوطة بالـ lease والـ fencing، ومرة واحدة لكل attempt، والـ masking في الـ runner قبل الـ spill والإرسال بكل الصيغ | الـ TLS اللي بيتقفل عند الـ proxy مش كفاية، والـ masking لازم قبل أي حاجة تخرج | [0045](ADR/0045-secrets-transport-and-masking.md) |
| 11 | الـ logs في `"LogChunks"` مقسّم بالأيام، وseq من غير فراغات بقفل الـ attempt، وSSE بـ `Last-Event-ID`، وNOTIFY بين الـ instances، والاحتفاظ بـ `DROP` للـ partition | شغّال مع أكتر من replica من غير blob store، والـ segments بتيجي M4 | [0046](ADR/0046-job-logs-storage-and-streaming.md) |
| 12 | `"Jobs"` و`"JobAttempts"` و`"Leases"` و`"AttemptSteps"`، وtrigger بيرفض أي انتقال مش في الرسم، وإعادة أوتوماتيك قبل أول خطوة mutating بس، وإلغاء SIGTERM ثم SIGKILL ثم cgroup kill، وreconciler بـ `SKIP LOCKED` | exactly-once للانتقالات من قاعدة البيانات نفسها | [0047](ADR/0047-job-state-machine-and-reconciliation.md) |
| 13 | systemd ووضع `runner` في الـ installer (engine مشترك في `internal/installer`)، وHelm chart واحد، وعلى Kubernetes pod لكل job بـ restricted، وnode pool لوحده موصى بيه | runner من غير privileges على Kubernetes، ومن غير Podman جوه pod | [0048](ADR/0048-runner-packaging-and-kubernetes.md) |

## قاعدة البيانات
كل الجداول فيها الأعمدة الأساسية الأربعة، والوقت `timestamptz`، والنص `TEXT`، والحذف soft (ما عدا partitions الـ logs). التفاصيل في الـ ADRs.

| الجدول | الموديول | الـ migration | أهم القيود |
|---|---|---|---|
| `"Runners"` | Runners | M3.2 | unique على `"SigningPublicKey"`، و`"State"` IN (`active`, `disabled`, `revoked`)، و`"Capacity"` من 1 لـ 16 |
| `"RunnerRegistrationTokens"` | Runners | M3.2 | unique على `"TokenHash"` (32 byte)، و`"Uses" <= "MaxUses" <= 50`، و`"ExpiresAt" <= "CreatedAt" + 24h` |
| `"RunnerSessions"` | Runners | M3.2 | unique على `"TokenHash"` و`"ChallengeNonce"`، وجلسة نشطة واحدة لكل runner (partial unique) |
| `"RunnerThrottles"` | Runners | M3.2 | نفس شكل `"LoginThrottles"` لـ `Register` و`Authenticate` |
| `"Jobs"` | Jobs | M3.3 | CHECK على الحالة، وtrigger للانتقالات، وindex على (`"OrganizationId"`, `"State"`, `"Priority"` DESC, `"QueuedAt"`) WHERE queued |
| `"JobAttempts"` | Jobs | M3.3 | unique على `"FencingToken"`، وattempt واحد غير نهائي لكل job، وtrigger للانتقالات |
| `"Leases"` | Jobs | M3.3 | unique على `"AttemptId"`، وindex على `"ExpiresAt"` WHERE active |
| `"AttemptSteps"` | Jobs | M3.3 | unique على (`"AttemptId"`, `"Index"`) |
| `"LogChunks"` | Jobs | M3.4 | `PARTITION BY RANGE ("CreatedAt")` يومي، و`PRIMARY KEY ("Id", "CreatedAt")`، و`octet_length("Data") <= 65536` |

- sequence اسمها `"FencingTokens"` (bigint).
- أرقام الـ migrations: الرقم الفاضي الجاي وقت الـ merge (حاليًا آخر رقم `00005`، وM2.2 لحد M2.5 ممكن ياخدوا أرقام قبل M3).

## الـ API

### REST (`packages/openapi/openapi.yaml`، والـ tags `runners` و`jobs`)
| Route | الإجراء | ملاحظة |
|---|---|---|
| `GET /organizations/{id}/runners` | `runner.read` | صفحات، والحالة المحسوبة (`online` لو `"LastSeenAt"` أقل من دقيقتين) |
| `GET /organizations/{id}/runners/{runnerId}` | `runner.read` | من غير أي مفتاح أو توكن |
| `PATCH /organizations/{id}/runners/{runnerId}` | `runner.manage` | `Name` و`Labels` و`Disabled` |
| `POST /organizations/{id}/runners/{runnerId}/revoke` | `runner.manage` | نهائي |
| `GET /organizations/{id}/runner-registration-tokens` | `runner.manage` | metadata بس |
| `POST /organizations/{id}/runner-registration-tokens` | `runner.manage` | `Token` بيرجع مرة واحدة، ومعاه أمر التسجيل |
| `POST /organizations/{id}/runner-registration-tokens/{tokenId}/revoke` | `runner.manage` | |
| `GET /organizations/{id}/jobs` | `job.read` | فلتر بالحالة |
| `POST /organizations/{id}/jobs` | `job.create` | `JobSpec` بنسخة M3 |
| `GET /organizations/{id}/jobs/{jobId}` | `job.read` | الـ attempts والخطوات والسبب لو `queued` |
| `POST /organizations/{id}/jobs/{jobId}/cancel` | `job.cancel` | idempotent |
| `GET /organizations/{id}/jobs/{jobId}/logs` | `log.read` | `Attempt` و`FromSeq` و`Limit` |
| `GET /organizations/{id}/jobs/{jobId}/logs/stream` | `log.read` | `text/event-stream`، و`Last-Event-ID` |

جدول الأدوار في [ADR-0047](ADR/0047-job-state-machine-and-reconciliation.md).

### الـ proto (`packages/proto/dhow/runner/v1`)
```
RegistrationService.Register(token, name, os, arch, version, isolation_levels[], capacity,
                             signing_public_key, encryption_public_key, proof)
  → runner_id, organization_id, labels[], server_signing_key, server_key_version

AuthService.Challenge(runner_id) → challenge (nonce, runner_id, expires_at, mac)
AuthService.Authenticate(runner_id, challenge, signature) → access_token, expires_in
AuthService.RotateKeys(new_signing_key, new_encryption_key, old_sig, new_sig) → key_version

LeaseService.Lease(free_slots, wait_seconds) → optional SignedLeaseGrant { grant_bytes, signature, key_version }
  LeaseGrant: lease_id, attempt_id, job_id, organization_id, runner_id, fencing_token,
              lease_ttl_seconds, issued_at, JobSpec
LeaseService.Start(LeaseRef, pop_signature) → SealedSecrets { kem, enc, ciphertext }, control
LeaseService.Heartbeat(LeaseRef) → control { cancel_requested, lease_ttl_seconds }
LeaseService.BeginStep(LeaseRef, index, mutating) → control
LeaseService.EndStep(LeaseRef, index, exit_code, failure_reason) → control
LeaseService.Complete(LeaseRef, result, exit_code, failure_reason, digest) → ok

LogService.Append(LeaseRef, chunks[] { seq, stream, step_index, emitted_at, data })
  → last_seq, expected_seq?, log_truncated, control

LeaseRef: lease_id, fencing_token
JobSpec:  image (digest), steps[] { name, run, env, mutating, timeout }, env, secrets[] { name, as_env | as_file, steps[] },
          resources { cpu_millis, memory_bytes, pids, disk_bytes }, timeout_seconds,
          network { egress[] }, isolation (container | gvisor), labels[]
```
- **بروتوكول الـ agent** (`packages/proto/dhow/agent/v1`): frames بطول مسبوق على قناة التحكم: `Hello` و`RunStep` و`Output` و`StepExit` و`Signal` و`AddMask` و`Shutdown`. داخلي بين الـ runner والـ agent، ومش عقد عام.
- **التحقق من `JobSpec`** في package مشترك `internal/jobspec` (السيرفر بيتحقق وقت الإنشاء، والـ runner بيتحقق تاني قبل التشغيل).

### الإعدادات الجديدة
- **السيرفر** (بتتضاف لـ `docs/install/configuration.md`، والاختبار بتاع الجدول بيمسك الناقص):
  - `DHOW_RUNNER_LISTEN_ADDR` و`DHOW_RUNNER_TLS_CERT_FILE` و`DHOW_RUNNER_TLS_KEY_FILE`.
  - `DHOW_RUNNER_MAX_POLLS` و`DHOW_JOB_LOG_MAX_BYTES` و`DHOW_LOG_RETENTION_DAYS` و`DHOW_SSE_MAX_STREAMS`.
- **الـ runner** (`/etc/dhow-runner/runner.env` من خلال `EnvironmentFile=`، وكل قيمة بيتحقق منها وبيفشل مقفول):
  - `DHOW_RUNNER_URL` و`DHOW_RUNNER_CA_FILE` و`DHOW_RUNNER_STATE_DIR`.
  - `DHOW_RUNNER_DRIVER` (`podman` أو `kubernetes`) و`DHOW_RUNNER_CAPACITY`.
  - `DHOW_RUNNER_MAX_CPU` و`_MAX_MEMORY` و`_MAX_PIDS` و`_MAX_DISK` و`_MAX_TIMEOUT` و`_DISK_MODE`.
  - `DHOW_RUNNER_CANCEL_GRACE` و`DHOW_RUNNER_EGRESS_ALLOW_PRIVATE_CIDRS` و`DHOW_RUNNER_ALLOW_UNPINNED_IMAGES` و`DHOW_RUNNER_RUNSC_PATH`.

## تقسيم الـ PRs
كل PR بيعدّي `make check` وCI أخضر، وفيه اختبارات سلبية لكل مسار أمني، وبيحدّث `Plan/Progress/` في يومه. الأوامر تحت بتفترض:

```
export DHOW_TEST_DATABASE_URL='postgres://dhow:dhow@127.0.0.1:5432/dhow_test?sslmode=disable'
docker run -d --name dhow-pg -e POSTGRES_USER=dhow -e POSTGRES_PASSWORD=dhow -e POSTGRES_DB=dhow_test -p 5432:5432 postgres:18
```

### M3.1: عقد الـ proto والـ codegen
- **المحتوى:**
  - `packages/proto` بالهيكل اللي في [ADR-0042](ADR/0042-runner-protocol-and-leasing.md): `buf.yaml` (v2، lint `STANDARD`، breaking `FILE`) و`buf.gen.yaml`.
  - كل ملفات `dhow/runner/v1` و`dhow/agent/v1` بالعقد كامل زي الـ outline فوق. من غير كومنتات، لأن `STANDARD` مش بيطلبها.
  - الكود المتولّد في `packages/proto/gen/go`.
  - **`Makefile`:**
    - `tools` بيبني `protoc-gen-go` (`google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.12`) و`protoc-gen-connect-go` (`connectrpc.com/connect/cmd/protoc-gen-connect-go@v1.21.0`) في `bin/tools`.
    - `gen` بيضيف `buf generate` (`github.com/bufbuild/buf/cmd/buf@v1.73.0`).
    - `proto-lint` جديد: `buf lint` و`buf breaking --against '.git#branch=main,subdir=packages/proto'`.
  - `go.mod`: `connectrpc.com/connect v1.21.0` و`google.golang.org/protobuf v1.36.12`.
  - الـ CI: `proto-lint`، و`make gen` ومعاه `git diff --exit-code`.
- **قبول:**
  - كل RPC بيغيّر حاجة فيه `LeaseRef`.
  - `LeaseGrant` بيتبعت `bytes` متوقّعة، مش message مفكوكة، عشان التوقيع يبقى على نفس الـ bytes بالظبط.
  - مفيش streaming RPC.
  - الكود المتولّد بيعدّي `go vet`.
- **التحقق:** `make gen && git diff --exit-code` و`make proto-lint` و`go build ./packages/...` و`make check`.

### M3.2: السيرفر: موديول `Runners` والهوية
- **الاعتماد:** M3.1.
- **المحتوى:**
  - **الـ migration** لـ `"Runners"` و`"RunnerRegistrationTokens"` و`"RunnerSessions"` و`"RunnerThrottles"`.
  - **`openapi.yaml`:** routes الـ runners والتوكنات (tag `runners`).
  - **`Core/Policy`:** الإجراءات `runner.read` و`runner.manage`، و`ScopeRunner` بأنواعه، و`policy.Runner(...)`.
  - **`Core/RunnerAuth`:**
    - gate بـ `connect.WithRequestGate` قبل فك الـ protobuf.
    - `Principal` فيه المؤسسة والـ labels والـ slots ومستويات العزل والمفاتيح العامة.
    - interface `Verifier` بيتركّب من `Runners` في `App`.
  - **تركيب `/runner-api/` في `App`** بره `/api/v1`، بـ middleware خاصة: RequestID وClientIP وAccessLog (من غير `Authorization`) وRecover وBodyLimit 1 MiB.
  - **الـ Connect handlers:** `RegistrationService` و`AuthService`.
  - **مفتاح الـ runner plane:** بـ HKDF من `DHOW_SECRET_KEY`، ومفاتيح الـ HMAC للتوكنات والـ challenges.
  - **الـ listener المنفصل:** `DHOW_RUNNER_LISTEN_ADDR` بـ TLS، والـ config بيفشل لو الملفات ناقصة أو صلاحياتها مفتوحة.
  - **الأحداث:** `runner.*` في `Core/EventBus` (audited)، و`runner.revoked` بيوصل لـ `Jobs` بعدين.
  - **حماية النسخ:** كشف الـ clone (أكتر من 3 تبديلات جلسة في 10 دقايق).
  - **الـ hooks:**
    - `check_route_policies` بيقبل `policy.Runner` وتركيب الـ Connect handler في `Runners.Routes.go` و`Jobs.Routes.go` بس، ومعاه اختبارات.
    - قاعدة الهيكل اتجاه الاستيراد لـ `Core/RunnerAuth`.
  - **اختبار الـ route-table** بيتوسّع للـ procedures.
- **قبول:**
  - توكن منتهي أو مستهلك أو ملغي أو من مؤسسة تانية بيرجّع `INVALID_REGISTRATION_TOKEN` بنفس الرد.
  - 50 طلب `Register` متوازي على توكن `MaxUses=3`: 3 runners بالظبط.
  - الـ runner ماقدرش يختار labels، والـ labels بتاعة القدرات بالـ prefix المحجوز بس.
  - توقيع `Authenticate` اتعاد: مرفوض. وchallenge أقدم من 60 ثانية: مرفوض.
  - revoke: أول RPC بعده `PERMISSION_DENIED`.
  - session cookie على `/runner-api/`: `UNAUTHENTICATED`. وrunner token على `/api/v1`: `401`.
  - مفيش مفتاح ولا توكن في أي response (اختبار بيمشي على كل الـ DTOs) ولا في الـ access log.
- **التحقق:**
  - `make gen && git diff --exit-code` و`make hooks-test`.
  - `go test -race -count=1 ./apps/server/internal/Modules/Runners/... ./apps/server/internal/Core/... ./apps/server/internal/App/...`
  - `make check`.

### M3.3: السيرفر: موديول `Jobs` والـ leases والـ reconciler
- **الاعتماد:** M3.2.
- **المحتوى:**
  - **الـ migration** لـ `"Jobs"` و`"JobAttempts"` و`"Leases"` و`"AttemptSteps"` والـ sequence والـ triggers.
  - **`internal/jobspec`:** التحقق والحدود والـ digest.
  - **routes الـ jobs** (create وlist وget وcancel)، والإجراءات `job.*`.
  - **`LeaseService`:**
    - `Lease` بـ long-poll و`LISTEN dhow_jobs_queued`، وحد الـ slots بـ advisory lock.
    - `Start` بالتحقق من التوقيع، و`SecretResolver`، والـ HPKE بـ `crypto/hpke`.
    - `Heartbeat` و`BeginStep` و`EndStep` و`Complete`.
  - **الـ reconciler** (كل 5 ثواني، وعند الإقلاع قبل الـ listener).
  - **قواعد الإعادة**، والإلغاء، والـ deadline، والـ queue timeout.
  - **الاشتراك في `runner.revoked`.**
  - **`SecretResolver`:** في الإنتاج بيرفض بـ `SECRETS_UNAVAILABLE`، وفي الاختبار من `Infra.Secrets`.
  - **الفرق في التنفيذ** ([ADR-0051](ADR/0051-jobs-and-leasing-implementation.md)): الـ migration رقمها `00031`، و`"Result"` اتشال (الحالة النهائية هي النتيجة، والسبب في `"FailureReason"`)، و`leased → failed` اتضاف لرسم الـ attempt والـ job (فشل قبل `Start`)، ورفض `RESOURCE_REQUEST_EXCEEDS_RUNNER` وقت الإنشاء اتأجّل لـ M3.5 لأن الـ runner لسه مابيبلّغش عن سقف موارده.
- **قبول:**
  - **انتقالات الحالة:**
    - اختبار table-driven لكل انتقال مسموح وممنوع، على مستوى الكود وعلى الـ trigger مباشرة بـ SQL.
    - 100 goroutine بـ 100 runner على job واحد: lease واحد.
    - `Complete` متكرر بنفس الـ digest: OK، وبـ digest مختلف: `ALREADY_EXISTS`.
  - **الـ fencing:**
    - RPC بـ fencing قديم بعد إعادة الـ lease: `LEASE_LOST`.
    - `Start` تاني: مرفوض.
  - **الإعادة:**
    - attempt ضاع قبل `BeginStep(mutating)` بيتعاد بـ fencing أكبر، وبعده الـ job بيبقى `lost` بـ `LOST_AFTER_MUTATION`.
    - runner ملغي ما بياخدش الـ job تاني.
  - **الإلغاء:**
    - إلغاء job `queued`: فوري.
    - إلغاء job `running`: `cancel_requested` في الـ heartbeat الجاي، و`BeginStep` بعده مرفوض.
  - **الأسرار:**
    - الـ bundle مابيتفكش بمفتاح runner تاني ولا بـ aad attempt تاني.
    - job فيه `SecretRef` في الإنتاج: `SECRETS_UNAVAILABLE` وقت الإنشاء.
  - **العزل بين المؤسسات:** runner من مؤسسة ب مابيشوفش jobs مؤسسة أ.
- **التحقق:**
  - `go test -race -count=1 ./apps/server/internal/Modules/Jobs/... ./internal/jobspec/...`
  - `go test -race -count=20 -run 'Lease|Race' ./apps/server/internal/Modules/Jobs/Test/Edge/...`
  - `make check`.

### M3.4: السيرفر: الـ logs
- **الاعتماد:** M3.3.
- **المحتوى:**
  - **الـ migration** لـ `"LogChunks"` المقسّم.
  - **الـ hook:** قاعدة الأعمدة الأساسية بتقبل `PRIMARY KEY ("Id", ...)` مع `PARTITION BY` بس، ومعاه اختبار إيجابي وسلبي.
  - **`LogService.Append`** بقفل الـ attempt، والسقف، والـ `system` line عند القص.
  - **صيانة الـ partitions** (3 أيام قدّام، و`DROP` بعد الاحتفاظ) تحت advisory lock، وفحص الـ partition الحالية في `/health/ready`.
  - **routes:** التاريخ والـ SSE.
  - **التوزيع:** hub بـ `LISTEN dhow_log_appended`، وحدود الـ streams، وفحص الجلسة كل 15 ثانية.
  - **الوثائق:** `docs/install/reverse-proxy.md` وملفات `deploy/proxy-check` (timeout الـ `/runner-api/`، و`proxy_buffering off` على مسار الـ stream). واختبار التطابق الموجود بيمسك الفرق.
  - **الفرق في التنفيذ** ([ADR-0052](ADR/0052-job-logs-implementation.md)):
    - الـ migration رقمها `00033`، لأن M2.4 بياخد `00032`.
    - الـ batch اللي فيه فراغ في النص بيتقبل منه الجزء المتصل، و`"LastSeq"` بقى أعلى seq اتأكد، وسطور السيرفر بتاخد الـ seq الجاي.
    - الحد 10 streams لكل مستخدم بدل لكل جلسة.
    - الصيانة بـ `pg_try_advisory_xact_lock` (الـ instance التانية بتتخطّى)، وفشلها وقت الإقلاع بيوقّفه.
    - الـ hook بيقبل `%I` في `CREATE TABLE` جوه `format()`.
- **قبول:**
  - **الترتيب والتكرار:** إرسال مكرر ومتلخبط من 3 goroutines بيطلّع log واحد مرتب من غير فراغات ولا تكرار. والفراغ بيرجّع `expected_seq`.
  - **SSE:**
    - قطع في نص الـ stream والرجوع بـ `Last-Event-ID`: نفس البايتات بالظبط.
    - الاتصال من instance تانية (اتنين `App` على نفس قاعدة البيانات) بيوصله نفس الـ chunks.
    - المشترك البطيء بيتقفل والباقيين مايتأثروش.
    - الجلسة الملغية بتقفل الـ stream في أقل من 16 ثانية.
  - **السقف:** بيقص ويكتب سطر `system` واحد.
  - **الاحتفاظ:** الـ partition الأقدم بيتعمله `DROP`، والـ partition الناقصة بتخلّي `/health/ready` يرجع `503`.
  - **الصلاحيات:** `log.read` ممنوعة على Viewer (route-table).
- **التحقق:**
  - `make hooks-test`.
  - `go test -race -count=1 ./apps/server/internal/Modules/Jobs/...`
  - `go test ./deploy/proxy-check/...` و`make proxy-check` (محلي).
  - `make check`.

### M3.5: الـ runner: الهيكل والهوية والـ lease loop والـ logs (بـ driver وهمي)
- **الاعتماد:** M3.3 (وM3.4 للـ logs).
- **المحتوى:**
  - `apps/runner/cmd/dhow-runner` و`apps/runner/internal/{App,Config,Identity,Client,Executor,Journal,Masking,Logs,Sandbox}`.
  - الـ hook بيتوسّع لهيكل `apps/runner/internal` (PascalCase واتجاه الاستيراد)، ومعاه اختبارات.
  - **`Config`:** من البيئة، وبيفشل مقفول (أي سقف أكبر من موارد الجهاز، أو ملف هوية صلاحياته مفتوحة، أو `http://` من غير `--dev` على localhost).
  - **الأوامر:** `register` (توكن من ملف أو stdin) و`run`.
  - **`Client`:** TLS صارم، وchallenge وتجديد التوكن عند ثلثين المدة، وbackoff بـ jitter.
  - **`Executor`:** التحقق من توقيع الـ grant، والـ journal (fsync، و`0600`)، والـ heartbeat، والـ deadline المحلي والقفل الذاتي، والإلغاء، والإنهاء بإعادة.
  - **`Masking`:** Aho-Corasick والصيغ كلها، ومعاه fuzz.
  - **`Logs`:** التقسيم لسطور، والـ chunker، والـ spill، والضغط على الـ pipe.
  - **`Sandbox`:** الـ interface، وdriver وهمي تحت build tag `dhowtest` بيرجّع output متسجّل ومابيشغّلش أي process.
  - **`make build-runner`**، والـ CI بيبني الـ runner.
  - **الفرق في التنفيذ** ([ADR-0053](ADR/0053-runner-implementation.md)):
    - الـ journal بيحفظ النتيجة لما تبقى معروفة، فالإقلاع بيبعتها هي بدل `lost` لو الـ runner وقع قبل الرد.
    - الـ masker بيحجز أطول بداية ممكنة لسر بس، بدل تداخل بطول أطول سر.
    - انتهاء وقت الخطوة بيقفل `failed` بـ `STEP_FAILED` (والخطوة `TIMED_OUT`)، لأن السيرفر بيرفض `failed` بسبب `TIMED_OUT`.
    - الـ E2E بيبني الـ runner ويشغّله كـ process، لأن قاعدة `internal` في Go بتمنع اختبار واحد يستورد الاتنين.
    - رفض `RESOURCE_REQUEST_EXCEEDS_RUNNER` وقت الإنشاء لسه متأجل (محتاج حقل في `RegisterRequest`).
- **قبول:**
  - **E2E (سيرفر حقيقي في نفس العملية وPostgres، والـ runner بالـ driver الوهمي):** تسجيل ثم lease ثم logs ثم إنهاء، والـ job `succeeded`.
  - **grant متوقّع بمفتاح تاني:** مرفوض ومش بيتنفّذ.
  - **القفل الذاتي:** proxy TCP بيعمل blackhole في الاختبار بيقطع السيرفر، والـ runner بيقتل الـ sandbox قبل `ExpiresAt` بـ 10 ثواني تقريبًا (تفاوت ±1 ثانية).
  - **الـ masking:** سر بالصيغ كلها في الـ output مش موجود في `"LogChunks"`، ولا في ملف الـ spill، ولا في الـ journal.
  - **الإعادة بعد الرستر:** الـ runner اترستر في نص job، فـ `Complete(lost, RUNNER_RESTARTED)` اتبعت بالـ fencing القديم.
  - **fuzz:** 60 ثانية لكل target من غير crash.
- **التحقق:**
  - `make build-runner` و`make hooks-test`.
  - `go test -race -count=1 -tags dhowtest ./apps/runner/...`
  - `go test -run '^$' -fuzz FuzzMasker -fuzztime 60s ./apps/runner/internal/Masking`
  - `make check`.

### M3.6: الـ sandbox: driver الـ Podman والـ agent والحدود (مستوى `container`)
- **الاعتماد:** M3.5.
- **المحتوى:**
  - **`Sandbox/Podman`:**
    - ملفات `containers.conf` و`storage.conf` و`registries.conf` بتاعتنا، بتتكتب في مجلد الحالة عند الإقلاع.
    - `create` بالـ flags اللي في [ADR-0043](ADR/0043-job-sandbox-rootless-podman.md)، وبعده `inspect` بيتأكد من كل حاجة.
    - الهدم وتنظيف الإقلاع.
  - **`Agent`:** PID 1، و`PR_SET_DUMPABLE=0`، وقناة التحكم، و`sh -e -s`، والـ process groups، والخروج لو القناة اتقفلت.
  - **ملف seccomp** (`Seccomp.json` كبيانات متضمّنة في الـ binary)، ومعاه اختبار بيقارن قايمة المقفول بـ ADR-0043.
  - **فحوص الإقلاع:** Podman 5.4+، وcgroup v2، وتفويض `cpu` و`memory` و`pids` و`io`، وsubuid، وuser namespaces.
  - **`apps/runner/Test/Isolation`:**
    - `escape-probe` (Go static) وimage اختبار بتتبني محليًا من `scratch`.
    - `make runner-isolation`.
  - **job في الـ CI اسمه `isolation`** على `ubuntu-26.04` (الـ image بتاعة `ubuntu-24.04` طلع فيها Podman 4.9.3، ADR-0055):
    - Podman من التوزيعة، والـ job بيتأكد إنه 5.4 أو أحدث، ولو أقدم بيفشل.
    - user مخصص بـ subuid.
    - الـ runner تحت `systemd-run -p Delegate=yes -p MemorySwapMax=0 --uid=...`.
- **قبول:** الـ escape suite والـ limits suite (تحت، من غير أجزاء الشبكة) خضرا، وأي probe بينجح يفشّل الـ job.
- **التحقق:**
  - محلي على Fedora أو Debian 13 أو VM: `make runner-isolation`.
  - في الـ CI: job `isolation`.
  - `make check`.

### M3.7: الشبكة: الـ egress proxy
- **الاعتماد:** M3.6.
- **المحتوى:**
  - **`Egress`:**
    - parser للقايمة (IDNA والـ wildcards والـ ports)، ومعاه fuzz.
    - CONNECT وabsolute-form.
    - الحل والفلترة بالنطاقات، والاتصال بالعنوان اللي اتحل.
    - سطور `system` متجمّعة، وحد الاتصالات وحد الخمول.
  - **forwarder في الـ agent** على `127.0.0.1:3128`، والـ helper `connect`، والـ helper `mask`.
  - **التحقق من القايمة** في `internal/jobspec` على السيرفر.
  - **`docs/pipeline-spec.md`:** `network.egress` بالقيود الجديدة.
- **قبول:**
  - **probes الشبكة:**
    - من غير egress: مفيش أي اتصال، ولا DNS.
    - مع egress: الـ host المسموح شغّال (سيرفر HTTPS في الاختبار على نطاق مفتوح في إعداد المشغّل بس).
    - الـ host الممنوع مرفوض، والـ port الممنوع مرفوض.
    - `169.254.169.254` و`[fd00:ec2::254]` مرفوضين حتى لو اتكتبوا في القايمة.
  - **DNS rebinding:** resolver اختبار بيرجّع عنوان عام وبعدين `127.0.0.1`، والاتصال التاني مرفوض.
  - **SSH:** `ssh -o ProxyCommand='/run/dhow/bin/connect %h %p'` لسيرفر SSH اختبار شغّال.
  - **fuzz:** 60 ثانية من غير crash.
- **التحقق:**
  - `go test -race -count=1 ./apps/runner/internal/Egress/... ./internal/jobspec/...`
  - `go test -run '^$' -fuzz FuzzAllowlist -fuzztime 60s ./internal/jobspec` و`go test -run '^$' -fuzz FuzzProxyRequest -fuzztime 60s ./apps/runner/internal/Egress` (الـ parser اتنقل لـ `internal/jobspec`، ADR-0057).
  - `make runner-isolation`.

### M3.8: gVisor (spike بشروط، وبعده التنفيذ)
- **الاعتماد:** M3.6 وM3.7.
- **المحتوى:**
  - `DHOW_RUNNER_RUNSC_PATH`، و`--runtime` و`--runtime-flag` المطلوبين (منهم `host-uds=open`).
  - تنزيل `runsc` بنسخة `release-20261005.0` ومعاه SHA-512 في الـ installer.
  - الـ runner بيعلن `gvisor` بس لو الـ spike checks عدّت عند الإقلاع.
- **شروط القبول (الـ go/no-go):**
  - الـ escape suite والـ limits suite كاملين على `isolation: gvisor`.
  - الحدود متطبّقة على الـ sandbox كله من بره (memory وpids على الـ sentry).
  - الـ egress socket شغّال.
- **لو الـ spike فشل:** النتيجة بتتكتب في `Plan/Progress`، وADR جديد بيأجل gVisor على الـ host، وM3.11 بيغطي gVisor على Kubernetes بس.
- **التحقق:** `make runner-isolation ISOLATION=gvisor` محلي. وفي الـ CI على `ubuntu-24.04` لو ثابت، وإلا job ليلي على VM.

### M3.9: الإلغاء بالتدريج والفوضى (chaos)
- **الاعتماد:** M3.7.
- **المحتوى:**
  - التدريج الكامل (`SIGTERM` ثم `SIGKILL` ثم `podman kill` و`cgroup.kill`).
  - `apps/runner/Test/Chaos` و`make runner-chaos`:
    - harness بيشغّل السيرفر كـ process حقيقي وPostgres والـ runner بـ Podman.
    - proxy TCP فيه أعطال.
    - «sink» HTTP بيسجّل `DHOW_FENCING_TOKEN` وفترة كل attempt، عشان نكشف أي تداخل.
  - job ليلي في الـ CI، وعلى أي PR عليه label اسمه `chaos`.
- **قبول (السيناريوهات):**
  1. **job بيتجاهل SIGTERM:** بيموت بـ SIGKILL بعد المهلة.
  2. **job بيعمل fork ويهرب من الـ process group:** بيموت بالـ cgroup kill. مفيش process فاضلة على الـ host (`ps` تحت `dhow-runner`).
  3. **`kill -9` للـ runner في نص خطوة غير mutating:**
     - الحاوية بتختفي في أقل من 3 ثواني (الـ agent خرج).
     - الـ runner بعد الرستر بينضّف ويبعت `RUNNER_RESTARTED`.
     - الـ job بيتعاد مرة واحدة وبينجح، والـ sink مافيهوش تداخل.
  4. **`kill -9` للـ runner بعد `BeginStep(mutating)`:** الـ job بيقفل `lost` بـ `LOST_AFTER_MUTATION`، ومفيش إعادة.
  5. **قتل السيرفر 20 ثانية (أقل من 35) وقيامه:** الـ job بيكمّل من غير ما يتعاد.
  6. **قتل السيرفر 90 ثانية:** الـ runner قفل نفسه في حدود 35 ثانية، والسيرفر لما قام علّم الـ attempt `lost` وأعاده، ومفيش تداخل.
  7. **blackhole للشبكة 60 ثانية:** نفس نتيجة 6. والـ RPCs القديمة بعد الرجوع بترجع `LEASE_LOST`.
  8. **رستر Postgres في نص job:** الـ RPCs بترجع `UNAVAILABLE`، والـ runner بيعيد، والـ job بينجح لو الرستر أقل من 35 ثانية.
  9. **إلغاء أثناء سحب الـ image:** الـ attempt بيبقى `canceled` من غير ما الحاوية تبدأ.
- **التحقق:** `make runner-chaos` (محلي على VM أو Fedora)، وjob `chaos` في الـ CI.

### M3.10: التغليف على الـ host
- **الاعتماد:** M3.6، والمرحلة ب في `Plan/Install-Guides.md` (لو لسه ماتعملتش، الـ PR ده بيبني الـ engine في `internal/installer`، والمرحلة ب بتستخدمه).
- **المحتوى:**
  - **`release.yml`:** أرشيفات `dhow-runner` لـ `linux/amd64` و`linux/arm64`، و`SHA256SUMS` وcosign وSBOM.
  - **`deploy/systemd/dhow-runner.service`.**
  - **`dhow-runner install` و`doctor` و`uninstall` و`rotate-keys`.**
  - **`get.sh --component runner`.**
  - **الوثائق:** `docs/install/runner.md`، وتحديث `docs/install/README.md` و`docs/install/configuration.md`.
  - **`deploy/test-vm/run.sh` و`make vm-test DISTRO=...`:** cloud image تحت QEMU/KVM.
- **قبول:**
  - **التنصيب:** `install --answers` على `ubuntu-24.04` في الـ CI (مع Podman الحديث اللي على الـ image)، وفي الـ VM على Debian 13 وFedora وRocky أو Alma 10، بيوصل لـ runner `online`. وبعده `doctor` نضيف، وjob تجريبي بينجح، و`uninstall` بيسيب الجهاز نضيف والـ runner `revoked` في السيرفر.
  - **الهوية مش صريحة على القرص:** `grep` على المفتاح الخاص في القرص مابيلاقيهوش لما `systemd-creds` يكون متاح.
  - **التقوية:** `systemd-analyze security dhow-runner.service` بيدّي `MEDIUM` أو أحسن، والإعداد اللي اتقبل متسجّل في الـ PR.
  - **الـ bootstrap:** `bash -n` وshellcheck نضاف على `get.sh`.
- **التحقق:** `make check` و`shellcheck deploy/install/get.sh` و`make vm-test DISTRO=debian-13` (وfedora وrocky-10)، وjob `runner-install` في الـ CI.

### M3.11: Kubernetes والـ Helm chart
- **الاعتماد:** M3.7.
- **المحتوى:**
  - **`Sandbox/Kubernetes`:** client بـ HTTPS من الـ standard library، وpod لكل attempt، وقناة التحكم بـ TLS وتوكن لمرة واحدة، والـ proxy على 3128، والتنضيف بالـ labels.
  - **`deploy/helm/dhow`** زي [ADR-0048](ADR/0048-runner-packaging-and-kubernetes.md).
  - **`make helm-lint`:** `helm lint` و`helm template | kubeconform -strict`.
  - **`make helm-e2e`:** kind، وPostgres كـ fixture بره الـ chart، والـ chart، وتسجيل، وjob.
  - **الوثائق:** `docs/install/kubernetes.md`.
- **قبول:**
  - **الـ chart بيرفض الإعدادات الناقصة:** من غير `database.existingSecret` أو `secretKey.existingSecret` أو TLS للـ runner plane، والسر المكتوب في الـ values.
  - **Pod Security:** كل الـ pods بتعدّي restricted (`kubectl label ns ... pod-security.kubernetes.io/enforce=restricted` قبل التنصيب).
  - **probes على pod الـ job:**
    - مفيش توكن ServiceAccount.
    - مفيش وصول للـ API server ولا لـ pods تانية ولا لـ `169.254.169.254`.
    - مفيش DNS.
  - **الـ NetworkPolicy:** اتصال ممنوع من pod job لـ Postgres بيفشل.
  - **الفوضى:** قتل pod الـ runner في نص job: pod الـ job بيخلص في أقل من 5 ثواني، والـ StatefulSet بيرجع بنفس الهوية، والـ job بيتعاد ويخلص.
  - **نسخ Helm:** `helm` 3.22.0 و4.3.0 الاتنين.
- **التحقق:** `make helm-lint` و`make helm-e2e`، وjob `helm` في الـ CI.

### M3.12: الواجهة: أجهزة التشغيل والـ jobs
- **الاعتماد:** M3.4، وتقسيم الصفحات (lazy routes) اللي اتطلب في M2.2، لأن الـ JS 171 من 180 KB.
- **المحتوى:**
  - **شريحة «أجهزة التشغيل»:** الحالة والـ labels، والتعطيل والإلغاء بتأكيد.
  - **شريحة «توكن تسجيل»:** بيظهر مرة واحدة، ومعاه أمر `get.sh` جاهز ونسخ. مابيتخزّنش في أي store بعد ما الشريحة تتقفل.
  - **قايمة الـ jobs، وشريحة job:**
    - الـ attempts والخطوات.
    - log حي بـ `EventSource`، والرجوع من آخر seq.
    - الـ ANSI بيتحوّل لـ spans، والعرض بحد 5000 سطر من غير virtualization.
    - زرار إلغاء.
  - **النصوص:** مصطلحات جديدة في `Plan/Glossary.md` (Job وAttempt وLease وRegistration token وLabel)، والمفاتيح في `Ar.ts` و`En.ts`.
- **قبول:**
  - lint وtypecheck والاختبارات والـ build والـ size في الميزانية.
  - Playwright على السيرفر الحقيقي وrunner بالـ driver الوهمي: عربي وإنجليزي وموبايل 390px، ومفيش خطأ في الـ console ولا مخالفة CSP.
  - التوكن مش في الـ URL ولا الـ history ولا `localStorage`.
- **التحقق:** `pnpm -C apps/web lint && pnpm -C apps/web typecheck && pnpm -C apps/web test && pnpm -C apps/web build && pnpm -C apps/web size`، و`make web` و`make check`.

### M3.13: التقوية وإثبات شرط الخلاص
- **الاعتماد:** كل اللي فات.
- **المحتوى:**
  - **targets fuzz إضافية:** decoder الـ frames في الـ agent، و`internal/jobspec`، وparser الـ CONNECT.
  - **مراجعة تهديدات للـ runner plane** بتتحدّث في `docs/security-model.md`.
  - **قايمة فحص للمراجعة الخارجية.**
  - **`deploy/m3-demo/run.sh`:** سيناريو شرط الخلاص كامل (عزل، وقتل runner، وترتيب) بيطبع النتيجة.
  - **`docs/roadmap.md`:** M3 خلص.
- **قبول:**
  - **الـ CI أخضر:** `isolation` و`chaos` و`helm` و`runner-install`.
  - **`deploy/m3-demo/run.sh`** نجح على VM نضيفة.
  - **الأسرار مش ظاهرة في أي حتة:** `grep` لقيمة السر في `pg_dump` وlogs السيرفر والـ runner وتخزين Podman والـ journal مالقاش حاجة.
- **التحقق:** `make check` و`make runner-isolation` و`make runner-chaos` و`make helm-e2e` و`deploy/m3-demo/run.sh`.

### الترتيب
```
M3.1 ─► M3.2 ─► M3.3 ─► M3.4 ─► M3.12
                  └──► M3.5 ─► M3.6 ─► M3.7 ─┬─► M3.8
                                  │          ├─► M3.9
                                  │          └─► M3.11
                                  └─► M3.10
                                                  M3.13 في الآخر
```

## الاختبارات: إيه بيشتغل فين

### الـ escape suite
`escape-probe` بيشتغل كـ job، وكل probe لازم **يفشل**:
- **الـ host:**
  - قراية `/etc/shadow` و`/proc/1/root` و`/proc/self/mountinfo` (مفيش mounts من الـ host).
  - symlinks من `/workspace` لبره.
  - الكتابة في `/sys` و`/proc/sys`.
  - تحميل kernel module.
- **credentials الـ runner:**
  - `/var/lib/dhow-runner` و`/etc/dhow-runner` و`environ` بتاع process الـ runner.
  - قناة تحكم job تاني.
  - التوكن والمفتاح في أي ملف أو متغير.
- **الـ jobs التانية:** jobين في نفس الوقت، كل واحد بيدوّر على علامة التاني في `/workspace` و`/tmp` و`/dev/shm` والـ abstract unix sockets والـ signals.
- **الـ metadata:** `169.254.169.254` و`fd00:ec2::254` و`metadata.google.internal`، مباشرة ومن الـ proxy.
- **الـ sockets:** `/var/run/docker.sock` و`/run/podman/podman.sock` و`$XDG_RUNTIME_DIR/podman/podman.sock` مش موجودين.
- **الصلاحيات:**
  - `CapEff` صفر في `/proc/self/status`.
  - `unshare -U` و`mount` و`keyctl` و`bpf` و`io_uring_setup` و`perf_event_open` و`ptrace` على الـ agent مرفوضين.
  - setuid جوه الـ image مابيرفعش الصلاحية.
  - raw socket مرفوض.
- **الشبكة:** اتصال مباشر من غير proxy، وDNS، وhost ممنوع، وport ممنوع، وDNS rebinding.
- **الأسرار:**
  - masking لكل الصيغ.
  - مفيش قيمة في `podman inspect` ولا في تخزين Podman ولا في الـ journal ولا في `pg_dump` ولا في logs السيرفر.

### الـ limits suite
| السيناريو | النتيجة المتوقعة |
|---|---|
| fork bomb | بيوقف عند `pids` والـ job `failed` بسبب واضح |
| استهلاك الذاكرة | `OOM_KILLED` |
| قياس الـ CPU | الـ throttling في `cpu.stat` متوافق مع الحد |
| ملء `/workspace` | `ENOSPC` |
| job بيعدّي وقته | `timed_out` بالتدريج |
| log بالجيجات | بيتقص عند السقف من غير ما الـ job يقف |
| سطر واحد 10 MiB | بيتقسم ويتقنّع صح |

### فين بيشتغل
| الاختبار | GitHub Actions | VM |
|---|---|---|
| السيرفر (state machine وleases وlogs وroute-table) | ✅ كل PR (Postgres 18 service) | — |
| الـ runner بالـ driver الوهمي وfuzz قصير | ✅ كل PR | — |
| escape وlimits على Podman rootless (`container`) | ✅ كل PR يلمس `apps/runner` أو `packages/proto` (`ubuntu-26.04`، وPodman من التوزيعة) | ✅ Fedora (SELinux enforcing) وDebian 13 وUbuntu 26.04 (قيود AppArmor على الـ userns) |
| gVisor على الـ host | لو الـ spike ثابت على `ubuntu-24.04` | ✅ ليلي |
| الفوضى (chaos) | ✅ ليلي وعلى label `chaos` | ✅ قبل كل release |
| Helm وKubernetes driver وNetworkPolicy | ✅ kind على كل PR يلمس `deploy/helm` أو `Sandbox/Kubernetes` | — |
| gVisor على Kubernetes | — | ✅ cluster حقيقي أو kind على VM فيها runsc، قبل كل release |
| الـ installer على التوزيعات | ✅ `ubuntu-24.04` | ✅ Debian 13 وFedora وRocky أو Alma 10 (ليلي) |
| fuzz طويل | — | ✅ ليلي (30 دقيقة لكل target) |

GitHub Actions بتوفّر KVM على الـ Linux runners العادية. فـ `make vm-test` بيشتغل هناك كمان، والـ jobs الليلية بتستخدمه. وأي VM خارجي (Hetzner مثلًا) مش لازم في M3.

## Edge cases
- **نسخ مجلد الهوية على جهازين:** الجلستين بيبدّلوا بعض، فالـ runner بيتقفل بـ `runner.disabled_clone_suspected` ومعاه audit.
- **الساعة:**
  - كل أوقات السيرفر من `now()` بتاع قاعدة البيانات.
  - الـ runner بيستخدم الساعة الـ monotonic للـ deadline، والساعة الحقيقية للعرض بس. فساعة غلط على الـ runner مابتكسرش الـ leases.
- **`Complete` بعد انتهاء الـ lease:** `LEASE_LOST`، والنتيجة بتتسجّل كـ log `system` على الـ attempt اللي ضاع (للتشخيص) من غير ما تغيّر الحالة.
- **job طالب `gvisor` ومفيش runner عنده:** بيفضل `queued` بسبب `NO_MATCHING_RUNNER` ظاهر، وبعد `QueueDeadline` بيبقى `failed`.
- **الـ image:**
  - digest مش موجود أو الـ registry واقع: `IMAGE_PULL_FAILED`، وده فشل بنية تحتية قبل أي mutation، فبيتعاد حسب القاعدة. والـ timeout بتاع السحب منفصل.
  - image من غير `/bin/sh`: الخطوة بتفشل `SHELL_NOT_FOUND` برسالة واضحة. والـ plugins (`uses:`) في M4 هي اللي مش محتاجة shell.
- **الـ output:**
  - output من غير newline لفترة طويلة: الـ chunker بيبعت كل 250 ms حتى من غير سطر كامل، والـ masker بيمسك آخر جزء لحد ما يتأكد إن مفيش سر متقسّم.
  - output binary أو UTF-8 بايظ: بيتخزّن bytes زي ما هو، وبيتحوّل لـ U+FFFD في القراية بس.
  - حروف التحكم في الاتجاه (bidi) في الـ logs: الواجهة بتعرض كل سطر في `dir="auto"` مع isolation، وعمرها ما بتحط HTML.
- **ملف الـ spill اتملى ومفيش قرص:** الـ runner بيوقف القراية (الـ job بيستنى)، والـ heartbeat شغّال. ولو القرص خلص فعلًا، الـ attempt بيفشل `RUNNER_DISK_FULL`، فشل بنية تحتية.
- **الـ partitions:**
  - مااتعملتش (قاعدة البيانات read-only مثلًا): `Append` بيرجّع `UNAVAILABLE` و`/health/ready` بيرجّع `503`، والـ runner بيعيد لحد ما الـ lease يقرّب يخلص.
- **Postgres failover في نص lease:** الـ connections بتتقطع والـ RPCs بترجع `UNAVAILABLE`. ولو الـ failover أقل من 35 ثانية، مفيش أثر.
- **توكن تسجيل `MaxUses=5` اتسرّب:** المهاجم بيسجّل runner بالـ labels بتاعة التوكن. الحماية: العمر القصير، والإلغاء، وظهور كل runner جديد في الـ audit والواجهة، وإن الـ labels بتاعة البيئات الحساسة تتعمل بتوكنات لمرة واحدة.
- **الـ runner والسيرفر على نفس الـ VPS:** الـ runner user مالوش وصول لملفات السيرفر (`LoadCredential` بتاع السيرفر). والـ job مايقدرش يوصل لـ `127.0.0.1:8080` بتاع السيرفر، لأن الـ loopback ممنوع في الـ proxy.
- **job بيطلب حدود أكبر من السقف:** بيترفض وقت الإنشاء لو مفيش runner في المؤسسة سقفه يكفي (`RESOURCE_REQUEST_EXCEEDS_RUNNER`)، ولو الـ runner اتغيّر بعدين، بيترفض في `Start` كفشل بنية تحتية من غير إعادة على نفس الـ runner.
- **image على Kubernetes محتاجة root:** مع `runAsNonRoot`، الـ pod بيترفض. الوثائق بتقول تستخدم image non-root، أو `hostUsers: false` على cluster بيدعمه.
- **SSE ورا proxy بيعمل buffer:** الـ comment كل 15 ثانية بيبيّن المشكلة في `doctor` بتاع السيرفر بعدين، والوثائق فيها `proxy_buffering off`.

## المخاطر
| الخطر | التخفيف |
|---|---|
| Podman 5.4+ مش في Debian 12 ولا Ubuntu 24.04 من مستودعاتهم | `doctor` بيقول بوضوح. Debian 13 وFedora وRHEL 9.6+ مدعومين. وSabry يقرر لو عايز مستودع خارجي |
| gVisor مع Podman rootless مش ثابت | spike بشروط في M3.8، والبديل RuntimeClass على Kubernetes |
| تفويض الـ cgroup (`Delegate=yes` مع `cgroupfs`) بيختلف بين التوزيعات | الـ runner مابيقومش من غير التفويض، واختبار VM لكل توزيعة. والبديل الموثّق `systemd --user` مع linger |
| قيود AppArmor على الـ user namespaces في Ubuntu الجديدة | فحص في `doctor`، واختبار على Ubuntu 26.04 في الـ VM |
| `NoNewPrivileges` ماينفعش على خدمة الـ runner | الـ runner مابيلمسش كود المستخدم بنفسه، والتقوية بتتقاس بـ `systemd-analyze security` |
| الـ tmpfs للقرص بيستهلك ذاكرة | السقف بيتحسب ذاكرة زائد قرص، و`disk_mode: directory` للأجهزة الصغيرة |
| الـ capabilities كلها متشالة: `apt-get` جوه الـ job بيفشل | مكتوب في الوثائق، وقرار capabilities محدودة في M4 بـ ADR |
| الـ egress بالـ proxy بس بيكسر برامج مابتعرفش proxy | الـ helper `connect` لـ SSH، والقيد مكتوب، وأي egress شفاف بـ ADR بعدين |
| الـ logs في Postgres بتكبّر قاعدة البيانات والـ backup | السقف والاحتفاظ، والـ segments في M4 |
| استثناء الـ hook للجداول المقسّمة | محصور في `PARTITION BY`، ومعاه اختبار |
| `connectrpc.com/connect` v1.21.1 فيه إصلاحات bugs وعمره أقل من أسبوعين | نبدأ بـ v1.21.0، ونرقّي لـ v1.21.1 بعد 2026-10-28 بـ Dependabot لو الـ CI أخضر |
| `crypto/hpke` في الـ standard library جديد نسبيًا | test vectors من RFC 9180 في الاختبارات، ونسخة Go مثبّتة في `go.mod` |
| أرقام الـ migrations بتتخبط مع M2.2 لحد M2.5 | الرقم بيتاخد وقت الـ merge |
| المرحلة ب في `Install-Guides.md` لسه ماتعملتش | M3.10 بيبني الـ engine المشترك لو احتاج |
| ميزانية الـ JS في الواجهة | M3.12 مستني تقسيم الصفحات، وممكن يتأجل لـ M7 من غير ما يأثر على شرط الخلاص |

## الاعتماديات المتوقعة (بنسخ مثبّتة)
اتأكدت يوم 2026-10-27 من proxy.golang.org (التاريخ) وOSV (مفيش advisories معروفة للنسخ دي)، وكلها أقدم من أسبوعين.

| الاعتمادية | النسخة | تاريخها | ليه |
|---|---|---|---|
| `connectrpc.com/connect` | v1.21.0 | 2026-09-08 | Connect للسيرفر والـ runner، وفيه `WithRequestGate` للمصادقة قبل فك الـ protobuf |
| `google.golang.org/protobuf` | v1.36.12 | 2026-08-10 | الـ runtime والـ `protoc-gen-go` |
| `github.com/bufbuild/buf` (أداة) | v1.73.0 | 2026-09-11 | lint وbreaking وgenerate. v1.74.0 وv1.75.0 أحدث من أسبوعين |
| `protoc-gen-connect-go` (أداة) | من connect v1.21.0 | 2026-09-08 | توليد الـ handlers والـ clients |
| `golang.org/x/sys` | v0.49.0 (موجودة indirect) | 2026-10-06 | `prctl` وsockets |
| gVisor `runsc` | `release-20261005.0` | 2026-10-08 | مستوى `gvisor`، بيتنزّل بالـ SHA-512 |
| Podman على جهاز الـ runner | 5.4 أو أحدث، من التوزيعة | — | مش بنثبّته احنا. `doctor` بيحذّر لو أقدم من آخر patch فيه إصلاح أمني معروف (آخر نسخ: 5.8.9 و6.1.4 في 2026-10-12) |
| kind (CI) | v0.33.0 | 2026-08-26 | Kubernetes في الـ CI، والـ node image متثبّت بالـ digest في الـ workflow |
| Helm (CI) | v3.22.0 وv4.3.0 | 2026-09-09 | `helm lint` والتنصيب بالاتنين. v3.22.1 وv4.3.1 أحدث من أسبوعين |
| kubeconform (CI) | v0.8.0 | 2026-06-04 | التحقق من الـ manifests |

**مش هنضيف:**
- `golang.org/x/net`: HTTP/2 في الـ standard library.
- مكتبة JWT أو PASETO: التوكنات opaque.
- `k8s.io/client-go`: ضخمة، وعمليات pods بس.
- مكتبة proxy: الـ standard library كفاية، ومن غير MITM.
- مكتبة Aho-Corasick: المتاحة مش متصانة، والخوارزمية صغيرة ومعاها fuzz.
- libseccomp: الملف JSON بيتمرر لـ Podman.
- الـ crypto كله من الـ standard library: `crypto/ed25519` و`crypto/hpke` و`crypto/hkdf` و`crypto/sha256`.

## قرارات مفوَّضة اتاخدت بالتوصية (Sabry يقدر يرجع فيها)
1. **Debian 12 وUbuntu 24.04:** مش مدعومين للـ runner من مستودعاتهم (Podman قديم). اتقرر: مش مدعومين في M3 بدل مستودع طرف تالت. الإصدارات المدعومة للـ runner: Debian 13 وUbuntu 26.04 وRHEL/Rocky/Alma 9.5+ و10 وFedora، وكلهم فيهم Podman 5.4 أو أحدث.
2. **`apt-get` جوه الـ job:** مش هيشتغل لأن الـ capabilities كلها متشالة. اتقرر: نسيبها كده في M3، ونقرر capabilities محدودة في M4.
3. **`POST /jobs` في M3:** لصاحب المؤسسة والمدير بس. اتقرر: يفضل كـ «تشغيل يدوي» بعد M4 بصلاحيات البيئة.
4. **Viewer:** مايقراش الـ logs ولا قايمة أجهزة التشغيل. اتقرر: كده، لأن الـ logs ممكن تكشف تفاصيل حتى بعد الـ masking.
5. **توكن التسجيل:** ممكن يتستخدم لحد 50 مرة لو الأدمن طلب. اتقرر: الافتراضي مرة واحدة، والحد 50 للأسطول.
6. **تغيير `DHOW_SECRET_KEY`:** بيكسر تسجيل الـ runners لحد M5. اتقرر: نقبله ونكتبه.
7. **الـ logs في Postgres لحد M4:** بالحذف بـ `DROP` للـ partitions كاستثناء من الـ soft delete. اتقرر: نقبله.
8. **الواجهة (M3.12):** ممكن تتأجل لـ M7 لو ميزانية الـ JS ضاغطة. اتقرر: نعملها لأن التسجيل محتاج واجهة لأي مستخدم حقيقي.
