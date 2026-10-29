[English](en/pipeline-spec.md)

# مواصفات خطوات النشر — `.dhow.yml` v1 (مسودة)

الحالة: **مسودة**. مخطط JSON Schema الذي تقرؤه البرامج سيكون في `packages/spec/`، ويشترك فيه السيرفر، وجهاز التشغيل، والـ CLI (`dhow lint`)، والمحرر المرئي.

## 1. المبادئ

- **مصدر واحد للحقيقة.** خطوات النشر (pipeline) في المستودع، عند الـ commit الذي يُنشر (ADR-0008).
  - للأحداث ذات الصلاحيات (النشر، وPRs من الـ forks)، نقرأ التعريف من **الفرع الأساسي/الافتراضي**، فلا يستطيع PR أن يعيد كتابة صلاحياته.
- **الخطوات معزولة.** الخطوة إما `run:` (أوامر لـ shell *داخل البيئة المعزولة للتشغيل*) أو `uses:` (إضافة في صورة حاوية).
- **التعبيرات بلغة CEL** داخل `${{ … }}`. لها أنواع، وليست Turing-complete، ولها حدود تكلفة.
  - القيم القادمة من Git (رسالة الـ commit، والكاتب، والفرع) **بيانات**، ولا نعيد تحليلها كتعبيرات أبدًا.
- **الأسرار بالإشارة، وليست مكتوبة.** `secrets.NAME` يُحل فقط داخل نطاق البيئة الخاصة بالتشغيل.
- **قيم `env` العادية ليست سرية.** كل عضو يستطيع قراءة التشغيل يراها في الـ API والواجهة، ومنهم المشاهدون. لذلك لا تضع أبدًا رمزًا أو كلمة سر أو مفتاحًا في `env`، بل أشر إليه من `secrets`.

## 2. مثال كامل

```yaml
version: 1
name: web-app

on:
  push:
    branches: [main]
  tag:
    patterns: ["v*"]
  pull_request: {}            # fork PRs: no secrets, no deploy environments
  artifact:                   # external CI hands over a built artifact
    from: github-actions

defaults:
  runner:
    labels: [linux, amd64]
    isolation: container        # container | gvisor | microvm
  timeout: 20m

concurrency:
  group: ${{ project.name + "-" + git.ref }}
  cancel_in_progress: true

jobs:
  test:
    image: node:22-bookworm@sha256:<digest>
    strategy:
      matrix:
        node: ["20", "22"]
    steps:
      - name: Install
        run: npm ci
      - name: Test
        run: npm test -- --ci
    cache:
      key: ${{ "npm-" + hash_files("package-lock.json") }}
      paths: [~/.npm]

  build:
    needs: [test]
    image: node:22-bookworm@sha256:<digest>
    network:
      egress: [registry.npmjs.org]      # default is deny-all
    steps:
      - run: npm ci && npm run build
    artifacts:
      - name: web
        paths: [dist/, package.json, package-lock.json]

  deploy-staging:
    needs: [build]
    if: ${{ git.branch == "main" }}
    environment: staging
    steps:
      - uses: dhow/deploy-ssh@sha256:<digest>
        with:
          artifact: web
          target: staging-vps          # defined on the environment, credentials are env secrets
          strategy: atomic-symlink
          keep_releases: 5
          reload: systemctl reload web-app
          health_check:
            url: https://staging.example.com/healthz
            expect_status: 200
            timeout: 60s

  deploy-production:
    needs: [deploy-staging]
    if: ${{ git.is_tag && git.tag.startsWith("v") }}
    environment: production            # protection rules enforced server-side (see §4)
    steps:
      - uses: dhow/deploy-ssh@sha256:<digest>
        with:
          artifact: web
          target: prod-fleet
          strategy: atomic-symlink
          rolling: { batch: 2, pause: 30s }
          health_check:
            url: https://example.com/healthz
      - uses: dhow/notify@sha256:<digest>
        with:
          channel: release-room
          message: ${{ "Released " + git.tag + " to production" }}

  migrate-db:
    needs: [deploy-production]
    environment: production
    steps:
      - run: ./bin/migrate up
        env:
          DATABASE_URL: ${{ secrets.DATABASE_URL }}
        id_token:
          audience: sts.amazonaws.com    # workload OIDC token instead of static cloud keys
```

## 3. المرجع (v1)

| المفتاح | النوع | ملاحظات |
|---|---|---|
| `version` | int | يجب أن يكون `1` |
| `on` | map | `push`، و`tag`، و`pull_request`، و`manual` (مع `inputs` لها أنواع)، و`schedule` (cron)، و`artifact`، و`api` |
| `defaults.runner` | map | `labels`، و`isolation` (أقل مستوى عزل، وسياسة البيئة قد ترفعه) |
| `concurrency` | map | `group` (CEL)، و`cancel_in_progress` |
| `jobs.<id>.needs` | list | حواف الـ DAG. نرفض الدوائر عند التحقق |
| `jobs.<id>.strategy.matrix` | map | الضرب الديكارتي. ندعم `include` و`exclude`، و`max_parallel` |
| `jobs.<id>.image` | string | يجب تثبيتها بالـ digest في بيئات الإنتاج (تفرضه السياسة). الاسم بلا عنوان سجل صور يُكمَل كما يفعل Docker: `node:22-bookworm@sha256:…` يعمل كـ `docker.io/library/node:22-bookworm@sha256:…`. جهاز التشغيل لا يبحث في سجلات صور أخرى أبدًا. |
| `jobs.<id>.environment` | string | يربط قواعد الحماية والأسرار والأهداف |
| `jobs.<id>.network.egress` | list | قائمة الأجهزة المسموحة. الفارغة تعني منع كل شيء، و64 عنصرًا بالأكثر. كل عنصر واحد من `host` أو `host:port` أو `*.domain`. (1) **الأجهزة:** حروف ASCII صغيرة، مع punycode (`xn--…`) للحروف الأخرى. نرفض عناوين IP، والأسماء المكوّنة من أرقام فقط، والأسماء من مقطع واحد (`localhost`، `*.com`). `*.domain` يطابق أي نطاق فرعي، لكن ليس `domain` نفسه. (2) **المنافذ:** بدون منفذ، يفتح العنصر 443 عبر `CONNECT` و80 لـ `http://` العادي. (3) **الوكيل:** تمر البيانات عبر وكيل جهاز التشغيل على `127.0.0.1:3128` (نضبط `HTTPS_PROXY` و`HTTP_PROXY` و`ALL_PROXY` لكل خطوة، وتغلب قيم التشغيل نفسه)، لذلك يجب أن تدعم البرامج الوكيل. يستطيع SSH وبرامج TCP الأخرى استخدام `ProxyCommand=/run/dhow/bin/connect %h %p`. (4) **غير مدعوم:** UDP، وQUIC، والبرامج التي تتجاهل الوكيل. (5) **ممنوع دائمًا:** العناوين الخاصة، وloopback، وlink-local، وعناوين metadata (ADR-0044، ADR-0057) |
| `jobs.<id>.if` / `steps[].if` | CEL bool | — |
| `steps[].run` | string | ينفّذه `/bin/sh -e` **داخل البيئة المعزولة** (أو `shell:`). نمرّر السكربت على stdin، ويجب أن توفّر الصورة الـ shell |
| `steps[].uses` | image ref | إضافة. يجب تثبيتها بالـ digest. `with:` فيه مدخلات لها أنواع، يعلنها ملف الإضافة |
| `steps[].env` | map | القيم يمكن أن تشير إلى `secrets.*` و`vars.*` |
| `artifacts` | list | رفع بعنونة المحتوى (content-addressed). يمكن تقليل `retention` حتى الحد الأدنى في سياسة المؤسسة |
| `cache` | map | `key` (CEL)، و`paths`. محدود لكل مشروع وفرع، وللقراءة فقط مع PRs من الـ forks |
| `timeout` | duration | لكل تشغيل أو خطوة. يقتله جهاز التشغيل بالقوة |

**سياق CEL.** هذه المتغيرات متاحة:
- `git.{sha, ref, branch, tag, is_tag, message, author, pr}`
- `project.*` و`env.*`
- `vars.*` (غير سرية)
- `secrets.*` (داخل `env`/`with` فقط، ولا يمكن أن تظهر في `if` أبدًا)
- `matrix.*` و`needs.<job>.outputs.*`

وهذه الدوال متاحة: `hash_files()`، و`startsWith()`، و`matches()` (بصيغة RE2)، و`contains()`.

## 4. حماية البيئات (في السيرفر، وليست في YAML)

تُضبط قواعد الحماية على البيئة من الواجهة أو الـ API، لذلك **لا يستطيع commit أن يضعفها**. القاعدة يمكن أن تحدد:
- الفروع أو الوسوم المسموحة؛
- المراجعين المطلوبين (العدد والفرق)، مع خيار منع الشخص من الموافقة على طلبه؛
- مؤقت انتظار ونوافذ نشر، ومنها فترات التجميد؛
- مستوى العزل المطلوب، واشتراط صور مثبّتة بالـ digest وإضافات موثّقة؛
- تجاوز للمدير، يُسجَّل في سجل الأحداث.

## 5. المحرر المرئي

تقدّم الواجهة محررًا مرئيًا لخطوات النشر (رسم للتشغيلات مع نماذج للخطوات)، موجّهًا للمستخدمين الجدد. يعمل هكذا:
- يقرأ `.dhow.yml` الحالي في الفرع الافتراضي.
- يعدّل نموذجًا له أنواع، ويتحقق منه مباشرة بمخطط JSON Schema نفسه.
- **يكتب YAML ويفتح pull request** عبر التكامل مع مزوّد Git. نسخة داو لا تبتعد عن المستودع أبدًا بصمت.

## 6. التحقق

`dhow lint` (الـ CLI)، والمحرر، والسيرفر، كلها تستخدم مدقق Go نفسه. يفحص المخطط، والـ DAG، وأخطاء الأنواع في CEL، والإضافات غير المعروفة أو غير المثبّتة، ومخالفات السياسة.
