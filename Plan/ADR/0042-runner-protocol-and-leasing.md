# ADR-0042: بروتوكول الـ runner: Connect بـ RPCs unary بس، وleases بـ fencing token، وتسليم at-least-once بإنهاء idempotent

- **الحالة:** مقبول (قرار مفوَّض في M3)
- **التاريخ:** 2026-10-27
- **بيكمّل:** [ADR-0005](0005-api-rest-openapi-and-connect.md) و[ADR-0006](0006-postgres-default-redis-optional.md) و[ADR-0041](0041-runner-identity-and-registration.md).

## السياق
- ADR-0005 قرّر Connect/gRPC فوق HTTP/2 للـ runners. لكن الـ runner بيوصل للسيرفر من خلال نفس الـ reverse proxy بتاع الواجهة. وnginx بيكلّم الـ upstream بـ HTTP/1.1 افتراضيًا، وفيه proxies كتير بتعمل buffer أو بتقطع الاتصالات الطويلة.
- `docs/architecture.md` بيقول إن الـ job اللي الـ runner بتاعه بطّل يبعت heartbeat بيبقى `lost`، والنظام عمره ما بيفترض إنه نجح. والنشر ما بيتعادش أوتوماتيك بعد أول خطوة بتغيّر في الـ target.
- محتاجين نقرر شكل `packages/proto`، ومعنى الـ lease بالظبط، وإيه اللي بيحصل لما الشبكة تتقطع.

## القرار

### النقل
- **connect-go** ببروتوكول Connect (مش gRPC ولا gRPC-Web)، والـ messages بـ protobuf binary.
- **RPCs unary بس في M3:** مفيش streaming في أي اتجاه. الـ unary في Connect شغّال على HTTP/1.1 وHTTP/2، فبيعدّي من أي proxy بإعداداته الافتراضية. الـ client بتاع Go بيستخدم HTTP/2 لو الـ ALPN سمح، وإلا HTTP/1.1. الـ long-poll بتاع `Lease` كمان unary بحد أقصى 25 ثانية، وده أقل من الـ timeouts الافتراضية في nginx (60) وCloudflare (100).
- **المكان:** الـ runner plane متركّب على `/runner-api/` في نفس الـ listener، بره `/api/v1` وبره الـ middleware بتاعه (الـ session والـ CSRF والـ timeout). المسار الكامل مثلًا `https://dhow.example.com/runner-api/dhow.runner.v1.LeaseService/Lease`.
- **listener منفصل (اختياري):** لو `DHOW_RUNNER_LISTEN_ADDR` موجود، الـ runner plane بينتقل له لوحده وبيختفي من الـ listener العام. الـ listener ده لازم بـ TLS (`DHOW_RUNNER_TLS_CERT_FILE` و`DHOW_RUNNER_TLS_KEY_FILE`) إلا لو `DHOW_DEV=true`. ده اللي Helm بيستخدمه عشان NetworkPolicy تفصل الـ port ([ADR-0048](0048-runner-packaging-and-kubernetes.md)).
- **الحدود:**
  - `ReadMaxBytes` واحد ميجا لـ `LogService.Append`، و256 KiB لأي حاجة تانية.
  - token bucket لكل runner في كل instance (50 طلب في الثانية، وburst لحد 100)، والزيادة بترجع `RESOURCE_EXHAUSTED` ومعاها وقت الاستنى.
  - long-poll واحد مفتوح بالكتير لكل runner.
- **الإصدار:** الـ runner بيبعت `Dhow-Runner-Version` و`Dhow-Protocol: 1`. السيرفر بيرفض بروتوكول مش مدعوم بـ `FAILED_PRECONDITION` وكود `RUNNER_UPGRADE_REQUIRED`.
- **مفيش `golang.org/x/net`:** الـ standard library بتدعم HTTP/2 (والـ h2c بـ `http.Server.Protocols` من Go 1.24).

### هيكل `packages/proto`
```
packages/proto/
  buf.yaml                     v2، lint STANDARD، وbreaking FILE ضد main
  buf.gen.yaml                 protoc-gen-go وprotoc-gen-connect-go (local plugins بنسخ مثبّتة)
  dhow/runner/v1/
    common.proto               RunnerRef وLeaseRef وأنواع الـ enums المشتركة
    registration.proto         RegistrationService: Register
    auth.proto                 AuthService: Challenge, Authenticate, RotateKeys
    job.proto                  JobSpec وStep وResources وNetwork وSecretRef وSealedSecrets وLeaseGrant
    lease.proto                LeaseService: Lease, Start, Heartbeat, BeginStep, EndStep, Complete
    logs.proto                 LogService: Append
  gen/go/dhow/runner/v1/       runnerv1 وrunnerv1connect (متولّد ومتعمله commit)
```
- `make gen` بيشغّل `buf generate` بعد الـ OpenAPI وsqlc، والـ CI بيتأكد إن مفيش فرق بعده. و`make proto-lint` بيشغّل `buf lint` و`buf breaking`. الـ breaking check بيشتغل من أول ما M3.2 يتعمله merge، وأي كسر مقصود قبل نهاية M3 بيتكتب في وصف الـ PR.
- أسماء الحقول في الـ proto بـ `snake_case` زي ما protobuf بيطلب، لأن الـ wire binary والأسماء مش بتظهر في JSON. قاعدة PascalCase في JSON بتاعة الـ REST API فاضلة زي ما هي.

### دورة الـ lease
1. **`Lease(free_slots, wait_seconds ≤ 25)`:** السيرفر بيدوّر على job في حالة `queued`، في نفس مؤسسة الـ runner، والـ labels بتاعته جزء من labels الـ runner، ومستوى العزل اللي طالبه موجود عند الـ runner. الاختيار بـ `FOR UPDATE SKIP LOCKED`، وفي نفس الـ transaction بيعمل attempt وlease. ولو مفيش، بيستنى إشعار `LISTEN dhow_jobs_queued` (اتصال واحد لكل instance بيوزّع في الذاكرة، ومعاه poll كل ثانيتين احتياطي). الاستنى عمره ما بيمسك transaction ولا connection.
2. **الرد `LeaseGrant`:** فيه `lease_id` و`attempt_id` و`job_id` و`fencing_token` و`runner_id` والـ `JobSpec` ومدة الـ lease. كله متسلسل bytes ومتوقّع بمفتاح الـ runner plane. الـ runner بيتحقق من التوقيع ومن إن `runner_id` هو هو قبل أي حاجة.
3. **`Start(lease, fencing, pop_signature)`:** بعد ما الـ runner يجهّز الـ sandbox (يسحب الـ image ويعمل الـ container). بيحوّل الـ attempt من `leased` لـ `running`، وبيرجّع الأسرار مشفّرة بـ HPKE مرة واحدة بس ([ADR-0045](0045-secrets-transport-and-masking.md)). أي `Start` تاني لنفس الـ lease بيترفض.
4. **`Heartbeat(lease, fencing)`** كل 10 ثواني، وبيمدّ الـ lease 45 ثانية من وقت قاعدة البيانات (`now()`، مش ساعة الـ instance).
5. **`BeginStep(lease, fencing, index, mutating)` و`EndStep(...)`:** حدود كل خطوة. الخطوة اللي بتغيّر في target (`mutating`، بتتعلّم من M6) ما تبدأش غير لما `BeginStep` يرجع OK، وساعتها السيرفر بيسجّل `"MutationStartedAt"`. ولو فيه إلغاء متطلب، `BeginStep` بيرفض، فمفيش تغيير بيبدأ بعد طلب الإلغاء.
6. **`Complete(lease, fencing, result, digest)`:** بيقفل الـ attempt.

### الـ fencing token
- رقم `bigint` من sequence واحدة على مستوى قاعدة البيانات، بيتدّي لكل attempt جديد. فأي attempt أحدث لنفس الـ job رقمه أكبر دايمًا.
- **كل RPC بيغيّر حاجة** (`Heartbeat` و`BeginStep` و`EndStep` و`Append` و`Complete`) بيتحقق في نفس الـ transaction:

  `WHERE "Id" = $lease AND "FencingToken" = $f AND "State" = 'active' AND "ExpiresAt" > now()`

  ولو مفيش صف، الرد `FAILED_PRECONDITION` بكود `LEASE_LOST`، والـ runner بيقتل الـ sandbox فورًا.
- الرقم بيوصل للـ job نفسه في `DHOW_FENCING_TOKEN`. فالـ targets في M6 تقدر ترفض أي كتابة برقم أقدم من آخر رقم شافته.

### التسليم والـ idempotency
- **at-least-once:** نفس الـ job ممكن يتأجّر أكتر من مرة لو attempt ضاع قبل أول خطوة `mutating` بس ([ADR-0047](0047-job-state-machine-and-reconciliation.md)). كل مرة attempt جديد بـ fencing جديد.
- **`Complete` idempotent:** لو الـ attempt اتقفل قبل كده بنفس الـ `digest` (SHA-256 للنتيجة)، الرد OK. ولو بنتيجة مختلفة، `ALREADY_EXISTS`. الـ runner بيعيد `Complete` بـ backoff لحد ما ياخد رد، أو لحد ما ياخد `LEASE_LOST`.
- **`Append` idempotent بالـ seq:** الـ chunks متسلسلة من غير فراغات ([ADR-0046](0046-job-logs-storage-and-streaming.md)). أي seq اتخزّن قبل كده بيتجاهل، وأي فراغ بيرجع `expected_seq`.
- **`Heartbeat` و`BeginStep` و`EndStep`** آمنين للإعادة.
- **الإشارات من السيرفر:** الـ `cancel_requested` ومدة الـ lease الجديدة بيرجعوا في رد كل `Heartbeat` و`Append`. فالـ job اللي بيطلّع output بيشوف الإلغاء في أقل من ثانية، والساكت بيشوفه في 10 ثواني بالكتير.

### الضغط (backpressure)
- **الـ runner:**
  - بيعلن عدد الـ slots، والسيرفر عمره ما بيأجّر أكتر منها. الحد بيتفرض في الـ claim نفسه بـ `pg_advisory_xact_lock` على الـ runner وعدّ الـ leases النشطة.
  - كل retry بـ exponential backoff مع full jitter، من ثانية لحد 30.
- **السيرفر:** لو الـ long-polls المفتوحة في الـ instance عدّت `DHOW_RUNNER_MAX_POLLS` (افتراضيًا 2000)، أو الـ pool بتاع قاعدة البيانات مشغول، بيرجّع `RESOURCE_EXHAUSTED` ومعاه `retry_after`.
- **الـ logs:** الـ runner عنده queue محدودة في الذاكرة وملف spill محدود. لما يتملوا، بيبطّل يقرا من الـ pipe، فعملية الـ job نفسها بتستنى على الـ write. ده ضغط طبيعي من غير ما نضيّع output، والـ wall-clock timeout شغّال برضه.

### انقطاع الشبكة
- **السيرفر:** الـ lease بينتهي عند `ExpiresAt`. والـ reconciler بيعلّمه `expired` بعد 5 ثواني سماح.
- **الـ runner بيقفل نفسه (self-fencing):** بيحسب deadline محلي بالساعة الـ monotonic: وقت **إرسال** آخر heartbeat نجح، زائد 45، ناقص هامش أمان 10. لو الـ deadline عدّى من غير heartbeat ناجح، بيقتل الـ sandbox بنفسه. وبما إنه بيحسب من وقت الإرسال، والسيرفر بيحسب من وقت الـ commit اللي بعده، الـ runner دايمًا بيقتل قبل ما السيرفر يعتبر الـ lease ضاع. فمفيش attempt جديد بيشتغل والقديم لسه شغّال.
- **لو السيرفر وقع وقام في أقل من 35 ثانية:** الـ heartbeats بتكمّل والـ job مابيتأثرش.
- **لو طوّل أكتر:** الـ runner قتل الـ job، والسيرفر لما يقوم بيلاقي الـ lease منتهي، فالطرفين متفقين من غير ما يكلموا بعض.
- **heartbeat اتعمله commit والرد ضاع:** الـ runner بيقتل بدري شوية، وده الاتجاه الآمن.

## البدائل اللي اتدرست
- **bidirectional stream واحد لكل runner (زي gRPC):** إشارات أسرع، لكنه محتاج HTTP/2 من أول الـ runner لحد السيرفر. وده مش مضمون ورا nginx الافتراضي ولا ورا proxies شركات، والفشل بتاعه أصعب في الفهم (الـ stream نص مقطوع).
- **server-streaming لإشارات الإلغاء:** بيحسّن زمن الإلغاء للـ job الساكت من 10 ثواني لأقل من ثانية، لكن محتاج `proxy_buffering off` على مسار تاني. ممكن نضيفه بعدين من غير ما نكسر البروتوكول.
- **gRPC بدل Connect:** محتاج HTTP/2 وtrailers، وبيتعطّل ورا proxies كتير.
- **lease من غير fencing:** الـ runner اللي رجع من انقطاع كان هيقدر يكتب logs أو نتيجة على attempt اتأجّر لغيره.
- **exactly-once delivery:** مستحيل مع runner ممكن يموت في أي لحظة. at-least-once مع إنهاء idempotent وحاجز قبل الخطوات اللي بتغيّر هو اللي بيدّي نفس النتيجة.

## العواقب
- كل RPC بيغيّر حاجة محتاج `lease_id` و`fencing_token`، وده جزء من الـ proto.
- زمن الإلغاء للـ job الساكت لحد 10 ثواني، ومكتوب في الوثائق.
- كل heartbeat كتابة في `"Leases"`، يعني 1000 runner بـ job واحد بيعملوا حوالي 100 كتابة في الثانية. `"Leases"` جدول لوحده عشان التحديثات المتكررة ما تنفخش `"JobAttempts"`.
- الـ proxy محتاج يسمح بطلبات لحد 30 ثانية على `/runner-api/`. وده بيتكتب في `docs/install/reverse-proxy.md` مع M3.4.
- `buf` وplugins الـ proto بقوا جزء من `make gen`.
