# ADR-0069: الـ parser والـ validator بتوع `.dhow.yml` (M4.1)

- **الحالة:** مقبول. القرارات 7 و8 و9 و13 مفوَّضة في [`M4-Pipelines.md`](../M4-Pipelines.md)، والباقي تفاصيل تنفيذ مفوَّضة ومكتوبة هنا.
- **التاريخ:** 2026-10-30
- **بيكمّل:** [ADR-0008](0008-pipeline-as-code-with-visual-editor.md). وبيبني على `internal/jobspec` بتاع [ADR-0051](0051-jobs-and-leasing-implementation.md) و[ADR-0057](0057-egress-proxy-implementation.md).

## السياق
- M4 محتاج package واحد بيقرا `.dhow.yml` ويتحقق منه ويحوّله لـ jobs، والسيرفر والـ runner والـ CLI (`dhow lint`) لازم يطلّعوا نفس النتيجة لنفس الملف.
- الملف جاي من مستودع، يعني مش موثوق: ممكن يبقى PR من fork، أو commit من حد سرّب توكن.
- الـ `JobSpec` بتاع M3 (`packages/proto/dhow/runner/v1/job.proto`) هو وحدة التنفيذ، ومفيهوش لسه `plugin` ولا `checkout` ولا `artifacts` ولا `cache` (دول إضافات M4.7 وM4.9).
- `CLAUDE.md` بيقول: شروط الـ pipeline بـ CEL بس، ومفيش مفسّر عام.

## القرار

### القرار 7: `${{ }}` ممنوع جوه `run:`
- أي `${{` في `run:` بيطلّع `RUN_EXPRESSION_NOT_ALLOWED`، والرسالة بتقول عدّي القيمة من `env:` واقراها كمتغيّر.
- مفيش escape ولا استثناء: `$${{` برضه مرفوض، لأن أي استثناء بيرجّع حقن الأوامر المشهور في GitHub Actions.
- القيمة اللي جاية من Git (رسالة الـ commit، واسم الفرع) بتوصل كمتغيّر بيئة، وعمرها ما بتبقى جزء من الـ script. الاختبار بيحط `$(touch /tmp/x) ${{ secrets.T }}` في رسالة الـ commit ويتأكد إن التقييم بيرجّعها نص زي ما هي.

### القرار 8: YAML بحدود ومن غير معاني مخفية
- **الحدود:** الملف 256 KiB، وأي scalar 64 KiB، والعمق 32، والـ nodes 50,000، وdocument واحد بس.
- **مرحلتين:** `go.yaml.in/yaml/v3` بيقرا لـ `yaml.Node` الأول، وبعدين فحص على الشجرة قبل أي decode:
  - الـ alias والـ anchor والـ merge key (`<<`) مرفوضين، وده بيقفل billion laughs.
  - الـ tags المسموحة هي الأساسية بس (`!!str` و`!!int` و`!!float` و`!!bool` و`!!null` و`!!map` و`!!seq`). `!!binary` و`!!timestamp` الصريح و`!!set` وأي tag خاص مرفوضين. التاريخ المكتوب عادي (`2026-10-30`) بيتقري نص.
  - المفتاح المكرر مرفوض، والمفتاح لازم يبقى قيمة بسيطة.
- **المفاتيح غير المعروفة:** فحص بالـ reflection على الشجرة نفسها ضد الـ `yaml` tags بتاعة الـ structs. ده بيعمل نفس شغل `KnownFields(true)`، بس بيدّي المسار والسطر والعمود، وبيفحص النوع (مثلًا `yes` مش boolean). بعد الفحص الـ decode من الشجرة مابيفشلش.
- **`pull_request:` من غير قيمة** معناه الحدث شغّال من غير فلاتر، فالـ null في struct pointer بيتحوّل لـ mapping فاضي قبل الـ decode.
- **الأخطاء:** كود ثابت `UPPER_SNAKE`، ومسار (`jobs.build.steps[2].run`)، وسطر وعمود، ورسالة إنجليزي ثابتة لكل كود. الرسالة عمرها ما بتكرر القيمة، ولا رسالة `yaml` ولا `cel-go` الأصلية بتطلع (ممكن فيها اسم متغيّر أو جزء من سر اتلزق بالغلط)، واختبار بيتأكد من ده. الترجمة للعربي في الـ CLI (M4.2) بالكود.

### القرار 9: الـ validator بـ Go هو المصدر
- `packages/spec/dhow.schema.json` (draft 2020-12) مكتوب بإيد للمحررات بس، ومفيش مكتبة JSON Schema في الـ runtime.
- `TestSchemaMatchesTheValidator` بيمشي على الـ structs بالـ reflection وعلى الـ schema مع بعض: كل مفتاح في الـ Go موجود في الـ schema والعكس، وكل object فيه `additionalProperties: false`، والأنواع متوافقة.
- الـ schema مابتغطيش كل حاجة (أنواع CEL، والـ DAG، والـ matrix، والحدود اللي بتتحسب)، ومكتوب ده في `description` بتاعها.

### القرار 13: `schedule` و`artifact` بيترفضوا
- الاتنين بيطلّعوا `TRIGGER_NOT_SUPPORTED_YET`، مش `UNKNOWN_KEY` ومش تجاهل: `schedule` بعد أول نسخة، و`artifact` مع M6.
- ونفس الفكرة لـ `isolation: gvisor` (`ISOLATION_NOT_SUPPORTED_YET`). أما `environment:` والأسرار فبيتقبلوا بـ warning (`ENVIRONMENTS_UNAVAILABLE` و`SECRETS_UNAVAILABLE`) زي الخطة، لأن الملف صح والميزة بس هي اللي جاية في M5.

### CEL
- **المكتبة:** `github.com/google/cel-go` v0.30.0 (2026-07-25). فيه v0.31.0، وv0.32.0 غيّرت الـ module path لـ `cel.dev/cel-go`، فقعدنا على النسخة اللي في الخطة. v0.30.0 فيها إصلاح GO-2026-6094 (مش بنستخدم `NativeTypes` أصلًا). بتجيب `cel.dev/expr` v0.25.1 و`antlr4-go/antlr/v4` v4.13.1 و`genproto`.
- **المتغيرات بأسماء فيها نقط** (`git.sha` و`git.pr.number` و`inputs.<name>` و`matrix.<key>` و`needs.<id>.outputs`)، مش maps من `dyn`. كده `git.branch == 1` و`inputs.x` اللي مش معلن و`needs.a` اللي مش في `needs` بيترفضوا وقت الـ check. و`vars` و`env` maps من string لـ string.
- **كل مكان ليه نطاق:** `if` بتاع الـ job مافيهوش `env`، و`concurrency.group` مافيهوش `matrix` ولا `needs`، واسم الـ artifact فيه `matrix` بس، و`hash_files()` في `cache.key` بس ومن غير implementation في السيرفر (الـ runner بيقيّمها).
- **التعبير هو القيمة كلها:** `${{ "v" + git.tag }}` مش `v${{ git.tag }}`. ده بيشيل محرّك templates كامل، وبيخلّي النوع واضح. `if` بيتكتب بـ `${{ }}` أو من غيره.
- **الأسرار مابتعدّيش على CEL:** `${{ secrets.NAME }}` مسموح بس كقيمة كاملة في `env` أو `with`، وبيتحوّل لـ `SecretRef` في الـ `JobSpec`. أي `secrets` في `if` بيطلّع `SECRETS_IN_IF`، وفي تعبير أكبر `SECRET_NOT_WHOLE_VALUE`، وفي `concurrency` أو `cache` `SECRETS_NOT_ALLOWED`. الفحص على الـ AST قبل الـ check، فحتى متغيّر comprehension اسمه `secrets` مرفوض.
- **الحدود:** 2 KiB لكل تعبير، وعمق الـ parser 32، و512 node، و`CostLimit` 10,000، والمقاطعة كل 64 لفّة، ومهلة 10 ms. اتجرّب: 100,000 كان بيوصل للمهلة قبل حد التكلفة. والتعبير المكلف بيقف في أقل من 50 ms حتى مع `-race`.
- **أخطاء التقييم** ليها أنواع ثابتة (`ErrExpressionCost` و`ErrExpressionTimeout` و`ErrExpressionEval`...) ومن غير نص `cel-go`.

### الـ DAG والـ matrix
- **`needs`:** Kahn بطابور مرتّب، فالترتيب ثابت. كل job في دورة بياخد `DAG_CYCLE`.
- **`include`:** كل عنصر تركيبة جديدة، ولازم يحدد كل مفاتيح الـ matrix (أو لو مفيش مفاتيح، كل العناصر بنفس المفاتيح). ده أبسط من سلوك GitHub (اللي بيوسّع التركيبات الموجودة)، وبيضمن إن كل `RunJob` ليه نفس متغيرات `matrix.*`. **`exclude`** بيستخدم مفاتيح الـ matrix بس.
- **الحدود:** 16 مفتاح، والحاصل قبل `exclude` 4096 بالكتير (عشان مانبنيش ملايين تركيبات)، وبعده 256 لكل job و256 للـ pipeline كله.
- **الاسم** `build (node=20, os=debian)` بالمفاتيح مرتّبة، والقيمة زي ما هي مكتوبة. تركيبتين بنفس الاسم `MATRIX_DUPLICATE`.
- **النوع في CEL:** لو كل قيم المفتاح من نوع واحد بياخده، ولو مختلطة `dyn` والنتيجة بتتفحص وقت التقييم (`ErrExpressionResult`).

### الحاجات الجديدة في الـ spec
- **`resources`:** `cpu` (`2` أو `1.5` أو `1500m`) و`memory` و`disk` (`Ki` و`Mi` و`Gi` و`Ti`) و`pids`، بنفس حدود `internal/jobspec`. المدد (`1h30m`) والأحجام بـ parser بتاعنا بحدود، مش `time.ParseDuration`.
- **المسارات** (`artifacts.paths` و`cache.paths`) نسبية لمساحة العمل بس: من غير `..` ولا `~` ولا `/` في الأول. فـ `~/.npm` في المثال القديم بقى `.npm` مع `npm ci --cache .npm`.
- **`with`** قيم بسيطة بس (نص أو رقم أو boolean)، فالـ maps المتداخلة في المثال القديم بقت مفاتيح مسطّحة (`health_check_url`). والـ `id_token` اتشال من المثال لحد M5.
- **صورة `uses:`** من غير digest خطأ (`USES_NOT_PINNED`)، و`image:` من غير digest warning (`IMAGE_NOT_PINNED`) زي الخطة.

### الـ compile («الخطة»)
- `Compile(spec)` بيرجّع `Plan` فيه `RunJob` لكل نسخة matrix بالترتيب الطوبولوجي، ولكل واحد `*runnerv1.JobSpec` متعمله `jobspec.Normalize` وبيعدّي `jobspec.Validate`.
- الحاجات اللي لسه مالهاش مكان في الـ proto بتفضل جنب الـ `JobSpec` في الـ `RunJob`: `Steps[i].Plugin` (الصورة والـ inputs)، و`Artifacts`، و`Cache`، و`Expressions` (التعبيرات اللي بتتقيّم وقت الإدراج بالـ `git` والـ `needs`). خطوة `uses:` في الـ `JobSpec` اسمها بس ومن غير `run`، والـ job اللي كله plugins من غير `image`، والفرقين دول بس اللي بيتسامح فيهم من `jobspec.Validate` لحد M4.7.
- **الحتمية:** `Digest` لكل `RunJob` هو sha256 لـ JSON (الـ maps مرتّبة) فيه الـ `JobSpec` بـ `proto.MarshalOptions{Deterministic: true}` وكل الحاجات اللي جنبه. اختبار بيعمل compile 20 مرة ويقارن، وfuzz بيتأكد.
- **التقييم** (`EvalIf` و`EvalStepIf` و`EvalValue` و`EvalGroup`) موجود ومختبَر، بس اللي بيستخدمه M4.8.
- `internal/jobspec` طلّع `ImageReason` و`ValidLabel` عشان الـ validator يستخدم نفس القواعد بالظبط ويدّي مكان الخطأ.

## البدائل اللي اتدرست
- **template في الـ strings** (`v${{ x }}`): أقرب لـ GitHub، بس محتاج parser للـ templates وقواعد escape، وكل جزء نوعه لازم يتحوّل لـ string. القيمة الكاملة كفاية مع `+`.
- **`git` وغيره كـ `map(string, dyn)`:** أسهل، بس أخطاء الأنواع بتظهر وقت التشغيل بس، وده عكس الخطة.
- **`ext.NativeTypes` على Go structs:** نوع قوي، بس محتاج struct tags غير `json`/`yaml` وكان فيه advisory عليه. الأسماء بالنقط أبسط ومن غير reflection.
- **مكتبة JSON Schema في الـ runtime:** مصدرين للحقيقة واعتمادية زيادة (القرار 9).
- **`yaml.Decoder.KnownFields(true)` لوحده:** رسايله فيها القيمة أحيانًا ومن غير مسار، فاتبدّل بالفحص على الشجرة.
- **أحدث cel-go (v0.32.0):** الـ module path اتغيّر، وده تغيير أكبر من المرحلة دي. نرجعله في ADR لوحده.

## العواقب
- السيرفر والـ runner والـ CLI بيستخدموا نفس الكود ونفس الأكواد، فـ `dhow lint` (M4.2) بيطلّع نفس أخطاء السيرفر.
- أي ميزة في الـ YAML محتاجة تعديل في الـ struct والـ schema مع بعض، والاختبار بيفشل لو واحد بس اتعدّل.
- `include` بتاعنا مش بيوسّع التركيبات زي GitHub، ومكتوب ده في `docs/pipeline-spec.md`.
- تقرا outputs الـ job بس لو الـ id بتاعه مافيهوش `-`، لأن `-` مش جزء من اسم في CEL. مكتوب في الوثيقة.
- الـ `if` بتاع الخطوة بيتقيّم قبل الإدراج، فمفيش لسه `failure()` ولا شروط على نتيجة خطوة قبلها.
- أسوأ ملف (25,000 تعبير تقريبًا) بياخد حوالي ثانية في الـ validation، واتقاس.
