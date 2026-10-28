# ADR-0058: آخر مرحلة في القتل بالتدريج، وsuite الفوضى (M3.9)

- **الحالة:** مقبول. فيه 3 قرارات خدها Sabry، والباقي مفوَّض.
- **التاريخ:** 2026-10-28
- **بيكمّل:** [ADR-0047](0047-job-state-machine-and-reconciliation.md) و[ADR-0053](0053-runner-implementation.md) و[ADR-0055](0055-podman-driver-and-agent-implementation.md) و[ADR-0057](0057-egress-proxy-implementation.md).

## السياق
ADR-0047 حدد إن الإلغاء بيمشي بالترتيب ده: `SIGTERM`، وبعده `SIGKILL`، وبعده `podman kill` و`cgroup.kill`. وقبل M3.9 كانت كل المراحل متنفّذة ما عدا `cgroup.kill`.

M3.9 كمان بيطلب إثبات إن الـ runner والسيرفر بيتعاملوا صح مع الأعطال الحقيقية. الأعطال دي هي: الإلغاء، و`kill -9` للـ runner، ووقوع السيرفر، وانقطاع الشبكة، ورستر Postgres، والإلغاء أثناء سحب الـ image. والإثبات لازم يبقى على processes حقيقية، مش على الـ driver الوهمي.

## القرار

### آخر مرحلة: `cgroup.kill`
- بعد `podman kill`، الـ Podman driver بيستنى لحد 5 ثواني لحد ما `cgroup.events` يقول `populated 0` على الـ cgroup بتاع الحاوية. ده نفس الـ cgroup اللي اتفحص وقت الـ start في ADR-0056.
- لو الـ cgroup لسه فيه processes، الـ driver بيكتب `1` في `cgroup.kill` وبيستنى 5 ثواني كمان.
- لو لسه فيه processes بعد كده، بيرجّع `sandbox.ErrSurvivors`. والـ executor بيقفل الـ attempt `SANDBOX_FAILED` بدل `canceled` أو `timed_out`، وبيكتب السبب في log الـ job.
- **ليه `SANDBOX_FAILED`:** process فاضل عايش معناه إن الـ sandbox ماعزلش زي ما المفروض. والسبب ده retryable بحد `MaxInfraRetries`، فالـ job ممكن يتعاد على runner تاني.
- **الـ cgroup مختفي:** لو Podman شاله، الـ driver بيعتبر إن الحاوية فاضية خلاص.
- **الـ sandbox ماقامش:** لو الـ start ماحصلش، مفيش cgroup معروف، فالمرحلة دي بتتعدّى.

### suite الفوضى: `apps/runner/Test/Chaos` (tag `chaos`)
- **processes حقيقية:**
  - `dhow-server` بـ `DHOW_DEV=true` وأسرار متولّدة، على Postgres 18.
  - `dhow-runner` بالـ Podman driver، تحت نفس user الـ isolation وبـ `Delegate=yes`.
  - الـ harness بيقتلهم ويقوّمهم بنفسه.
- **التجهيز من الـ API الحقيقي (قرار Sabry):**
  - الـ setup من الرابط اللي بيظهر في log السيرفر.
  - session، وتوكن تسجيل.
  - `dhow-runner register --token-file -`، وبعدين `run --dev`.
  - الـ jobs بتتعمل وبتتلغي وبتتقري من `/organizations/{id}/jobs`.
- **Postgres والـ registry:** حاويتين بـ Podman العادي بتاع الـ user (مش storage الـ runner)، فالـ harness يقدر يعمل رستر لـ Postgres من غير صلاحيات زيادة.
- **registry بـ TLS (قرار Sabry):**
  - الـ harness بيولّد CA مؤقتة، وشهادة لـ `localhost` و`127.0.0.1`.
  - الـ CA بتتحط في `certs.d` تحت `HOME` بتاع الـ runner، يعني مجلد الحالة.
  - مفيش أي تغيير في كود الإنتاج، والـ runner بيسحب بـ `docker://` وبالـ digest زي الإنتاج بالظبط.
  - `localhost:5001` طريق تاني لنفس الـ registry، بيعدّي على proxy بيبطّأ النقل لـ 100 KiB في الثانية. ده لسيناريو الإلغاء أثناء السحب، بـ image فيها 8 MiB عشوائية.
- **proxy TCP بأعطال:** بين الـ runner والسيرفر. بيعرف يعمل blackhole، يعني يستقبل الاتصالات ومايبعتش أي حاجة، ويبطّأ النقل.
- **الـ sink:** سيرفر HTTP على الـ host اسمه `sink.dhow.test` في `/etc/hosts`.
  - كل job بيناديه مرة كل ثانية من خلال الـ egress، ومعاه `DHOW_ATTEMPT_ID` و`DHOW_FENCING_TOKEN`.
  - التداخل معناه إن attempt قديم نادى الـ sink بعد ما attempt أحدث منه بدأ.
- **العمليات الفاضلة:** الـ harness بيدوّر في `/proc` على أي process الـ uid بتاعه من نطاق الـ subuids (200000 لـ 1199999). ده نطاق الـ user namespaces بتاعة الـ jobs بس.

### الـ CI (قرار Sabry)
- workflow جديد اسمه `chaos.yml` على `ubuntu-26.04`. بيشتغل كل ليلة الساعة 02:17 UTC، وبـ `workflow_dispatch`، وعلى أي PR عليه label اسمه `chaos` ومش draft.
- مابيشتغلش على كل PR، لأنه بياخد من 10 لـ 15 دقيقة، وسيناريو واحد بس فيه 90 ثانية انتظار.
- `passt` اتضاف للأدوات، عشان Podman بتاع الـ user يقدر ينشر بورتات Postgres والـ registry. والـ user `dhowci` بـ linger، عشان `XDG_RUNTIME_DIR` يبقى موجود.

## العواقب
- كل سيناريو في `Plan/M3-Runner.md` بقى ليه اختبار على processes حقيقية.
- أي كسر بيبان في الليلة اللي بعد الدمج، أو قبل الدمج لو الـ PR اتحط عليه label `chaos`.
- **مش متغطّي:**
  - **Kubernetes وgVisor:** هيتغطّوا في M3.11 وM3.8.
  - **أكتر من instance للسيرفر:** هيتغطّى مع الـ HA.
  - **انقطاع الشبكة بين السيرفر وPostgres:** السيناريو بيغطي الرستر بس.
