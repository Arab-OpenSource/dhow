# ADR-0053: تنفيذ الـ runner في M3.5: الطبقات، والهوية على القرص، والقفل الذاتي، والـ journal، والـ masking والـ logs، والـ driver الوهمي

- **الحالة:** مقبول (قرار مفوَّض في M3.5)
- **التاريخ:** 2026-10-28
- **بيكمّل:** [ADR-0041](0041-runner-identity-and-registration.md) و[ADR-0042](0042-runner-protocol-and-leasing.md) و[ADR-0043](0043-job-sandbox-rootless-podman.md) و[ADR-0045](0045-secrets-transport-and-masking.md) و[ADR-0046](0046-job-logs-storage-and-streaming.md) و[ADR-0047](0047-job-state-machine-and-reconciliation.md) و[ADR-0050](0050-runner-identity-implementation.md) و[ADR-0051](0051-jobs-and-leasing-implementation.md) و[ADR-0052](0052-job-logs-implementation.md).

## السياق
الـ ADRs اللي فاتت حددت البروتوكول من ناحية السيرفر: الهوية والـ grant الموقّع والـ fencing والـ heartbeat وعقد الـ `Append`. M3.5 هو أول مرة الطرف التاني بيتكتب، والتنفيذ طلّع أسئلة لازم تتقفل قبل M3.6 (Podman والـ agent): الـ runner بيتقسم لطبقات إزاي، وملف الهوية شكله إيه، والـ deadline المحلي بيتحسب من أنهي لحظة بالظبط، والـ journal فيه إيه، وإيه اللي بيحصل في كل فشل بين الـ grant والـ `Complete`، والـ masker بيمسك قد إيه، والـ logs بتعمل إيه لو السيرفر مابيردّش.

## القرار

### الطبقات (`apps/runner/internal`)
| الطبقة | الدور | تستورد |
|---|---|---|
| `App` | الأوامر (`register` و`run` و`version`) وحلقة الـ lease واختيار الـ driver | كل الطبقات |
| `Executor` | attempt واحد من الـ grant لحد `Complete`، والقفل الذاتي، والإعادة بعد الرستر | `Client` و`Identity` و`Journal` و`Logs` و`Masking` و`Sandbox` و`Config` |
| `Client` | Connect والـ TLS والتوكن والـ backoff | `Identity` و`Config` |
| `Logs` | التقطيع والطابور والـ spill والإرسال | `Masking` و`Config` |
| `Identity` | المفاتيح وملف الهوية | `Config` |
| `Journal` و`Sandbox` | الـ journal، والـ interface والـ drivers | `Config` |
| `Masking` و`Config` | الـ masker، والإعدادات وقواعد الملفات | ولا حاجة |

- مفيش حاجة بتستورد `App`، ومحدش غير `App` بيستورد `Executor`، والـ runner عمره ما بيستورد كود السيرفر (العقد المشترك في `internal/jobspec` و`internal/runnersig` و`packages/proto`).
- الـ hook (`runner_rules.py`) بيفرض الجدول ده وأسماء PascalCase، ومعاه اختبارات في `test_runner_hooks.py`، منها اختبار بيمشي على شجرة `apps/runner/internal` الحقيقية كلها.

### الإعدادات (`Config`)
- كل حاجة من البيئة (`/etc/dhow-runner/runner.env`)، وكل غلطة بتتجمع وبتوقف الإقلاع. القيم والحدود في `docs/runner/configuration.md`.
- **السقوف الافتراضية** هي القيم الافتراضية بتاعة `internal/jobspec` (ذاكرة 2 GiB وقرص 4 GiB و1024 process)، والـ CPU كله، والوقت 6 ساعات (ADR-0043). يعني job بالقيم الافتراضية بيشتغل على أي runner بإعداداته الافتراضية.
- **بيفشل مقفول لو أي سقف أكبر من الجهاز:** الـ CPU من `NumCPU`، والـ PIDs من `pid_max`، والذاكرة من `sysinfo`. ومع `DISK_MODE=tmpfs` الذاكرة زائد القرص لازم يكفّوا الذاكرة (ADR-0043، صفحات الـ tmpfs بتتحسب على الذاكرة)، ومع `directory` القرص لازم يكفّي المساحة الفاضية تحت `STATE_DIR`. مفيش تقليل صامت للسقف.
- **مجلد الحالة:** لو موجود لازم يكون مجلد بتاع نفس المستخدم ومحدش تاني يكتب فيه، و`identity` و`journal` لازم ملفات عادية (الـ symlink مرفوض) بتاعة نفس المستخدم ومن غير أي صلاحية لحد تاني. ده بيتفحص في `Load` نفسه، فـ `run` مابيبدأش.
- **العنوان:** `https://` بس، ومن غير path ولا query ولا credentials. `http://` مسموح بس مع `--dev` وعلى `localhost` أو `127.0.0.1` أو `[::1]`.
- **`DHOW_RUNNER_CA_FILE`** بيستبدل roots النظام، مش بيضيف عليها، فالـ runner بيثق في الـ CA ده بس.
- **الـ origin** (ADR-0050): origin بتاع `DHOW_RUNNER_URL` افتراضيًا، و`DHOW_RUNNER_SERVER_ORIGIN` بيغيّره لما الـ runner plane على listener لوحده. وبيتثبّت في ملف الهوية وقت التسجيل، و`run` بيستخدم المتثبّت دايمًا.
- `DHOW_RUNNER_EGRESS_ALLOW_PRIVATE_CIDRS` و`DHOW_RUNNER_RUNSC_PATH` و`DHOW_RUNNER_ALLOW_UNPINNED_IMAGES` بيتحقق منهم من دلوقتي (وبيفشلوا مقفولين)، واستخدامهم مع M3.7 وM3.8 وM3.6.

### الهوية على القرص
- `identity` ملف JSON (بأسماء PascalCase)، فيه رقم الإصدار والـ IDs وعنوان السيرفر والـ origin ومفتاح السيرفر المتثبّت ورقمه، والـ seed بتاع Ed25519 والمفتاح الخاص بتاع X-Wing بصيغة RFC 9180.
- بيتكتب atomic: ملف مؤقت `0600` في نفس المجلد، وfsync، وrename، وfsync للمجلد.
- `register` بيرفض لو فيه هوية، وبيكتب الهوية بعد رد السيرفر بس. لو الكتابة فشلت بعد التسجيل، الرسالة بتقول للمشغّل يلغي الـ runner ويسجّل تاني.
- `run` بيستخدم `DHOW_RUNNER_URL` لو موجود، وإلا العنوان اللي اتسجّل بيه.
- الـ process بيعمل `PR_SET_DUMPABLE=0` و`RLIMIT_CORE=0` قبل ما يقرا أي مفتاح (ADR-0045).

### `register`
- `dhow-runner register --url <server> --token-file <path|-> [--name] [--dev]`، زي الأمر اللي السيرفر بيطبعه.
- أي `--token` أو `--registration-token` أو قيمة بتبدأ بـ `dhow_rrt_` على سطر الأوامر بترجّع خطأ بيشرح ليه، حتى قبل ما الـ flags تتقري. والتوكن من الملف أو stdin لازم بشكل `dhow_rrt_` و16 لـ 200 حرف base64url، ومابيتسجّلش في أي log.
- الـ runner بيبلّغ عن العزل من الإعداد مش من الـ driver (`container`، و`gvisor` لو `RUNSC_PATH` موجود مع Podman)، فالتسجيل شغّال حتى قبل ما الـ driver الحقيقي يوصل.

### `Client`
- TLS 1.2 أو أحدث، ومفيش `InsecureSkipVerify` في أي مكان. والـ redirect مرفوض (`CheckRedirect`)، عشان ردّ مزوّر مايبعتش التوكن لعنوان تاني. `HTTPS_PROXY` مسموح لأن الـ TLS بيكمّل من الآخر للآخر جوه الـ CONNECT.
- كل طلب فيه `Dhow-Protocol: 1` و`Dhow-Runner-Version` من interceptor واحد.
- **التوكن:** challenge، وبيتأكد إن الـ challenge لنفس الـ `RunnerId`، وتوقيع `AuthMessage` بالـ origin المتثبّت. التجديد عند ثلثين العمر من لحظة قبل الـ challenge، وبيحصل وقت أول طلب بعدها (من غير goroutine). و`UNAUTHENTICATED` على أي RPC بيعمل `Authenticate` تاني مرة واحدة بس، ولو التوكن اللي فشل هو الحالي بس، فمفيش عاصفة تبديلات تشغّل كشف النسخ (ADR-0050).
- **الـ backoff:** عشوائي بين ثانية و`min(30s, 2^n s)`، والـ `Retry-After` بتاع السيرفر بيكسب (لحد 5 دقايق).
- `RUNNER_REVOKED` و`RUNNER_UPGRADE_REQUIRED` بيوقفوا الـ runner بخطأ. و`RUNNER_DISABLED` (كشف النسخ) والـ credentials المرفوضة بيستنوا بالـ backoff، عشان الـ runner يرجع لوحده لما الأدمن يفكّه.

### التحقق من الـ grant
- بالترتيب، وقبل أي حاجة: رقم مفتاح السيرفر هو المتثبّت، والتوقيع على `GrantMessage(grant_bytes)` بالمفتاح المتثبّت، وبعدين الفك، و`RunnerId` و`OrganizationId` بتوعنا، والـ IDs بصيغة UUID، والـ fencing موجب، والـ spec موجود، ومدة الـ lease من 20 ثانية لـ 5 دقايق.
- **الـ grant المرفوض مالوش أي أثر:** مفيش journal ولا sandbox ولا أي RPC عليه، لأن الـ `lease_id` اللي فيه نفسه مش موثوق. بيتسجّل خطأ في log الـ runner بس. والسيرفر بيلاقي الـ lease انتهى بعد 45 ثانية من غير heartbeat، وبيطبّق قاعدة الإعادة. ولو الـ grants كلها بتترفض، الـ job بيخلص `lost` بـ `RETRIES_EXHAUSTED`، وده الصح: فيه حد في النص أو مفتاح اتغيّر.
- **الـ spec بيتحقق تاني** بـ `jobspec.Validate` (`SPEC_REJECTED`)، والموارد والوقت قدّام السقوف (`RESOURCE_REQUEST_EXCEEDS_RUNNER`)، والعزل قدّام الـ driver (`SANDBOX_FAILED`). كلهم قبل `Start` ومن غير ما الـ sandbox يتعمل، والسيرفر بيعيد الأخيرين على runner تاني (ADR-0051).

### القفل الذاتي
- **الـ deadline** = آخر لحظة إرسال لطلب نجح ومدّ الـ lease، زائد المدة، ناقص 10 ثواني. الطلبات اللي بتمدّ هي `Heartbeat` و`Start` (ADR-0051)، وأول deadline من لحظة إرسال `Lease` نفسه، لأن الـ lease اتعمل جوه الطلب ده. ودايمًا بناخد الأبعد، فرد قديم مابيقرّبش الـ deadline.
- **الـ heartbeat** كل `min(10s, (المدة - 10s) / 3)`، يعني 10 ثواني مع الـ 45، وفي `leased` كمان. الفشل بيعيد كل ثانية، والطلب نفسه مابيعدّيش الـ deadline.
- **القتل** من `time.AfterFunc` على الـ deadline مباشرة، مش من حلقة الخطوات، فـ RPC معلّق أو خطوة شغّالة ماتأخّرهوش. والقتل `Kill` على الـ sandbox كله من غير مهلة، والـ sandbox اللي بيخلص تحضيره بعد القتل بيتقتل على طول.
- **بعد القفل:** الـ runner بيحاول `Complete(lost, LEASE_LOST)`. لو السيرفر رجع في الـ 10 ثواني دول، النتيجة بتتطبّق والـ job بيتعاد بدري. ولو لأ، النتيجة بتفضل في الـ journal للإقلاع الجاي.
- **`LEASE_LOST` من السيرفر** (في الـ heartbeat أو أي RPC): قتل فوري ومفيش `Complete`، لأن السيرفر قرر خلاص.
- الرد اللي ضاع بعد ما الـ heartbeat اتعمله commit بيخلّي الـ runner يقتل بدري بفاصل heartbeat واحد، وده الاتجاه الآمن (ADR-0042). اختبار الـ blackhole بيقطع بين heartbeatين عشان يقيس الـ 10 ثواني بالظبط.

### الـ journal
- `journal` ملف JSON `0600` بنفس الكتابة الـ atomic، فيه لكل lease: `LeaseId` و`AttemptId` و`JobId` و`FencingToken`، وبعد ما النتيجة تبقى معروفة `Completion` (أرقام النتيجة وكود الخروج والسبب). مفيش spec ولا env ولا اسم خطوة ولا أي output، والاختبار بيتأكد من شكل الملف كله.
- الـ lease بيدخل الـ journal بعد التحقق من الـ grant وقبل أي حاجة تانية. ولو الكتابة فشلت، الـ job مابيشتغلش والنتيجة `failed` (`RUNNER_DISK_FULL` لو القرص مليان).
- بيخرج بعد أي رد نهائي على `Complete` (OK أو `LEASE_LOST` أو `COMPLETION_CONFLICT` أو أي رفض مش مؤقت).
- **عند الإقلاع:** الـ driver بينضّف الـ sandboxes بتاعة الـ runner، ومجلدات الـ attempts بتتمسح، وبعدين لكل lease في الـ journal: لو `Completion` موجود بيتبعت هو (نفس الـ digest، فالإعادة آمنة)، وإلا `Complete(lost, RUNNER_RESTARTED)` بالـ fencing القديم. وبعدين بس بيطلب leases.
- الفرق عن ADR-0047: النتيجة المعروفة بتتبعت بدل `lost`، عشان runner وقع بعد ما الـ job نجح ما يحوّلوش لـ `lost`.
- journal بايظ بيوقف الإقلاع برسالة بتقول إن فيه IDs بس وممكن يتشال (fail closed).
- **الإيقاف (`SIGTERM`):** الـ runner بيبطّل يطلب، وبيقتل الـ sandboxes، وبيبعت `Complete(lost, RUNNER_RESTARTED)` في 15 ثانية بالكتير، واللي ماوصلش بيفضل في الـ journal.

### من الـ grant لـ `Complete`
- **`Start`** بيتعاد بالـ backoff على الفشل المؤقت. ولو رجع `ATTEMPT_ALREADY_STARTED` بعد فشل مؤقت (الـ commit حصل والرد ضاع)، الـ job بيكمّل لو مافيهوش أسرار، ولو فيه بيفشل `SANDBOX_FAILED` لأن الأسرار مش هترجع تاني (ADR-0045). و`CANCEL_REQUESTED` بيقفل `canceled`، و`SECRETS_UNAVAILABLE` بيقفل من غير `Complete` لأن السيرفر قفل الـ attempt.
- **الأسرار:** الـ bundle بيتفك بالـ AAD بتاع الـ lease والـ fencing والـ attempt والـ runner، وكل قيمة بتدخل الـ masker قبل أول سطر output. والسر اللي أقل من 4 حروف بيطلّع سطر `system` بالاسم بس. وسر ناقص من الـ bundle أو KEM غريب: `SANDBOX_FAILED`. والقيم بتتمسح من الذاكرة في الهدم (best effort).
- **الـ env بتاع الخطوة:** env الـ job وبعده env الخطوة، و`DHOW_FENCING_TOKEN` (المتأجل من ADR-0051) و`DHOW_JOB_ID` و`DHOW_ATTEMPT_ID` و`DHOW_STEP_INDEX`، والأسرار في الخطوات اللي في `Steps` بتاعتها بس (الفاضية = كل الخطوات).
- **الإلغاء** (من رد `Heartbeat` أو `Append` أو أي RPC): `SIGTERM`، وبعد `CANCEL_GRACE` `SIGKILL`، وبعد 5 ثواني `Kill` للـ sandbox كله، وبعدين `EndStep(CANCELED)` و`Complete(canceled)`.
- **الوقت:** نفس التدريج. وقت الـ job بيخلّص `timed_out`. ووقت الخطوة بيخلّص `EndStep(TIMED_OUT)` و`Complete(failed, STEP_FAILED)` بكود الخروج، لأن السيرفر بيرفض `failed` بسبب `TIMED_OUT` (ADR-0051)، و`timed_out` معناه الـ deadline بتاع الـ job بس.
- كل الأوقات المحلية بالساعة الـ monotonic (`time.Now` بتاع Go).

### الـ masking
- Aho-Corasick مكتوب بإيدينا: edges متربة لكل node وجدول كامل للـ root بس، فالذاكرة على قد الأنماط.
- **الصيغ لكل سر:** القيمة، وbase64 العادي والـ URL-safe بالـ padding ومن غيره وبالتلات إزاحات، و`QueryEscape` و`PathEscape`، وhex صغير وكبير، وJSON بتلات أشكال (بالـ HTML escaping زي Go، ومن غيره، و`\/`)، وJSON بـ `\uXXXX` لكل حرف مش ASCII (زي Python)، وكل سطر لوحده بنفس الصيغ لو السر متعدد السطور. وأي صيغة أقل من 4 bytes بتتساب.
- **الاستبدال:** أي byte جوه أي match بيتغطّى، والتغطيات المتلاصقة أو المتداخلة بتبقى `***` واحدة، حتى عبر حدود الكتابات.
- **الحجز:** الـ stream بيحجز أطول suffix لسه ممكن يكمّل pattern بس، مش طول أطول سر ناقص واحد. ده بيحل محل «التداخل» بتاع ADR-0045 للسطور الطويلة، وبيخلّي الـ output من غير أسرار يطلع على طول. والإثبات: أي match بيبدأ بعد آخر نقطة طلعت، فمفيش جزء من سر بيعدّي.
- `AddMask` (من الـ agent في M3.6) جاهز: سر جديد بيتطبّق على اللي لسه محجوز وعلى اللي جاي.
- رسائل الأخطاء اللي الـ executor بيكتبها في log الـ runner بتعدّي على الـ masker، عشان أخطاء Podman (M3.6) ممكن يبقى فيها output.

### الـ logs
- الترتيب: الـ masker، وبعدين الـ chunker، وبعدين الطابور. يعني الـ spill والإرسال بيشوفوا نص متقنّع بس.
- **الـ chunk:** 64 KiB بالكتير، بيتقطع عند آخر newline لو فيه، وإلا عند حد حرف UTF-8 (ADR-0052)، وكل 250 ms لأي جزء ناقص. والـ seq من 1 لكل attempt على كل الـ streams.
- **الطابور:** 8 MiB في الذاكرة، وبعدين ملف spill لحد 256 MiB بصيغة (طول + protobuf `LogChunk`)، بـ `O_EXCL|O_NOFOLLOW` و`0600` في مجلد الـ attempt (`0700`). الملف FIFO وبيتقص لصفر لما يفضى، وبيتمسح مع الـ attempt. ولما الاتنين يتملوا، `Write` بيستنى، فالـ pipe بيقف (ADR-0042).
- **الـ batch:** 768 KiB بيانات و1024 chunk بالكتير، عشان الطلب يفضل تحت 1 MiB بتاع السيرفر حتى مع chunk زيادة وتغليف.
- **عقد ADR-0052:** الـ chunks اللي `seq` بتاعها أقل من `last_seq` أو قده بتتشال، والباقي بيتبعت من `expected_seq` لو موجود. ولو `last_seq` عدّى آخر seq عندنا (سطر السيرفر)، العدّاد بيقفز بعده. وبعد `log_truncated` الـ runner بيكمّل يقرا ويرمي.
- **الفشل:** المؤقت (ومعاه `UNAVAILABLE` بتاع الـ partition الناقصة) بيتعاد بنفس الـ seq. و`LEASE_LOST` بيقفل الـ attempt. و`UNIMPLEMENTED` (سيرفر من غير logs) بيرمي الـ output بتحذير. وأي رفض تاني بيوقف الإرسال للـ attempt ده من غير ما يوقف الـ job. و`ENOSPC` في الـ spill: الـ job بيتقتل و`failed` بـ `RUNNER_DISK_FULL`.
- **قبل `Complete`:** الـ runner بيستنى الطابور يفضى لحد `min(30s, الـ deadline - 1s)`، والـ heartbeat شغّال، فالـ lease مابيخلصش وهو بيستنى. واللي مااتبعتش بيضيع ويتسجّل تحذير.

### الـ sandbox
- الـ interface زي ADR-0043: `Prepare` و`Start` و`Exec` و`Signal` و`Kill` و`Destroy` و`Inspect`، و`Cleanup` للإقلاع. و`Exec` بياخد الـ env والملفات والـ writers، والأسرار عمرها ما بتعدّي على إعدادات الـ driver.
- **الـ driver الوهمي** (`Sandbox/Fake`، build tag `dhowtest`): مابيشغّلش أي process. بيقرا recording بصيغة JSON (`DHOW_RUNNER_FAKE_RECORDING`، والمتغيّر ده موجود في نسخ الاختبار بس) فيه output كل خطوة وكود خروجها ومدة تعليق وتجاهل `SIGTERM`، وممكن يطبع قيمة متغير env بصيغة (base64 وhex وurl وjson) عشان يثبت إن السر وصل للخطوة واتقنّع.
- **النسخة العادية مالهاش driver:** `run` بيفشل مقفول برسالة بتقول إن Podman جاي في M3.6 وKubernetes في M3.11، و`DHOW_RUNNER_DRIVER=fake` نفسه مرفوض فيها.

### البناء والاختبار
- `make build-runner` (Linux، static)، و`make test-runner` (`-race -tags dhowtest`)، و`make lint-runner` (golangci-lint بالـ tag)، و`make fuzz-runner` (6 targets، `FUZZTIME` افتراضيًا 60 ثانية). و`make check` فيه الـ lint والاختبار بالـ tag، والـ CI بيبني الـ runner وبيعمل fuzz 10 ثواني لكل target.
- **الـ E2E في `Modules/Jobs/Test/E2E`** بيبني الـ binary بـ `go build -tags dhowtest` وبيشغّله كـ process، لأن قاعدة `internal` في Go بتمنع package واحد يستورد `apps/server/internal` و`apps/runner/internal` مع بعض. وده كمان بيختبر الأوامر والبيئة والصلاحيات الحقيقية. السيرفر في نفس process الاختبار ورا `httptest` بـ TLS، والـ runner بيثق في شهادته بـ `DHOW_RUNNER_CA_FILE`.
- **الاختبارات جوه الـ runner** بتستخدم `App/Testing`: سيرفر Connect وهمي بيطبّق العقد (ومنه `Append` بتاع ADR-0052) بمفتاح توقيع وExpiresAt حقيقيين، وproxy TCP بيعمل blackhole.

## قيود معروفة
- **رفض `RESOURCE_REQUEST_EXCEEDS_RUNNER` وقت الإنشاء** (المتأجل من ADR-0051) لسه متأجل: محتاج حقل في `RegisterRequest` وعمود في `"Runners"` وفحص في `Jobs`. الـ runner بيرفض قبل `Start` والسيرفر بيعيد على runner تاني، فالنتيجة صح لكن متأخرة.
- **`rotate-keys`** و`systemd-creds` و`doctor` مش في M3.5 (M3.10).
- **الـ masker مابيمسكش** سر بيطلع بطريقة مش في القايمة (ADR-0045).
- **runner بيتعاد تشغيله أكتر من 3 مرات في 10 دقايق** وجلسته لسه صالحة هيتقفل كأنه نسخة (ADR-0050).
- **الـ chunks اللي كانت في الطريق** وقت قطع طويل بتضيع بعد 30 ثانية من آخر الـ job، والـ log بيبقى ناقص من آخره. البديل كان إن الـ attempt يستنى أكتر والـ slot يفضل محجوز.

## البدائل اللي اتدرست
- **الـ E2E جوه شجرة الـ runner بسيرفر في process تاني:** كان هيحتاج بناء السيرفر وتشغيل الـ setup والدخول من بره، والسيرفر الحقيقي في نفس الاختبار أقرب وأسرع.
- **package مكشوف من الـ runner (من غير `internal`) عشان السيرفر يستورده:** كان هيكسر الهيكل اللي في الخطة.
- **الحجز بطول أطول سر ناقص واحد:** أبسط، لكنه بيأخر أي output لحد الـ 250 ms أو سطر جديد حتى لو مالوش علاقة بأي سر.
- **الـ journal بـ IDs بس من غير النتيجة:** كان هيحوّل job نجح لـ `lost` لو الـ runner وقع بين النهاية والرد.
- **`Complete` للـ grant المرفوض:** محتاج نصدّق `lease_id` من رسالة توقيعها غلط.
- **مسح الـ spill في الذاكرة بدل الملف:** كان هيخلّي انقطاع طويل مع output كتير يا يستهلك ذاكرة من غير حد، يا يوقف الـ job بدري.
- **التجديد بـ goroutine بالوقت:** التجديد وقت الاستخدام أبسط، والـ runner بيعمل طلب كل 25 ثانية على الأقل.

## العواقب
- M3.6 بيكتب driver الـ Podman ورا نفس الـ interface، والـ agent بيستخدم `Masker.Add` لـ `AddMask`. ومحتاج يعدّي أخطاءه على الـ masker (اتعمل هنا) ويرجّع `SHELL_NOT_FOUND` و`OOM_KILLED` في `StepResult`.
- أي تغيير في صيغة ملف الهوية أو الـ journal رقم إصدار جديد، والقديم بيفضل بيتقري أو بيوقف الإقلاع بوضوح.
- الـ CI بقى بيبني الـ runner وبيشغّل اختباراته بالـ tag، والـ E2E بتاع السيرفر بيبني الـ runner كمان.
