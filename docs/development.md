[English](en/development.md)

# تطوير داو على جهازك

تأخذك هذه الصفحة من نسخة جديدة من المستودع إلى سيرفر يعمل، وواجهة مع إعادة التحميل المباشر، ومجموعة الاختبارات الكاملة. هناك طريقان:

- **[Docker فقط](#أسرع-بداية-docker-فقط):** أمر واحد، ولا تثبّت شيئًا غير Git وDocker. ابدأ من هنا.
- **[الأدوات المباشرة](#سكربت-الإعداد):** Go وNode.js وpnpm وPostgreSQL على جهازك (عبر WSL2 على Windows)، لأجل `make check`، والبيئة المعزولة لجهاز التشغيل، وأدوات Go وTypeScript في محررك.

لتجربة الصورة المنشورة بدلًا من ذلك، راجع [شغّل داو على جهازك](install/local.md#حزمة-compose).

> [!WARNING]
> كل شيء هنا يعمل مع `DHOW_DEV=true`: HTTP العادي، واتصال بقاعدة البيانات بدون TLS. هذا لجهازك أنت فقط. لا تضبطه أبدًا على سيرفر أو أي شيء يصل إليه غيرك.

## أسرع بداية: Docker فقط

كل ما تحتاجه Git وDocker: [Docker Desktop](https://www.docker.com/products/docker-desktop/) على Windows وmacOS، أو Docker Engine مع Compose plugin على Linux. يعمل Go وNode.js وpnpm وPostgreSQL في حاويات، فلا تثبّت أي شيء آخر، ولا يحتاج Windows إلى إعداد WSL بنفسك. في الطرفية (PowerShell على Windows):

```sh
git clone https://github.com/Khaymah/dhow.git
cd dhow
docker compose -f deploy/dev/compose.yaml up
```

التشغيل الأول ينزّل الصور (حوالي 600 MB) واعتماديات Go والواجهة، فيأخذ بضع دقائق. التشغيلات التالية تأخذ ثوانٍ. عندما يعمل كل شيء، يعرض السجل إطارًا مثل هذا:

```text
==================================================================
  Dhow is ready. Open this link within one hour to create the owner:
  http://localhost:5173/setup#...
==================================================================
```

افتح هذا الرابط في Chrome أو Firefox لتنشئ حساب صاحب النسخة. بعد ذلك استخدم `http://localhost:5173`.

- **Go:** احفظ أي ملف تحت `apps/server` أو `internal` أو `packages/proto/gen/go`، فيعيد السيرفر البناء والتشغيل خلال ثوانٍ. إذا فشل البناء، يظهر الخطأ في السجل، ويستمر السيرفر القديم في العمل، والحفظ التالي يحاول مرة أخرى.
- **الواجهة:** احفظ أي ملف تحت `apps/web` فيتحدث المتصفح في مكانه.

| الخدمة | ماذا تفعل | على جهازك |
|---|---|---|
| `db` | PostgreSQL 18 مع قاعدتي البيانات `dhow` و`dhow_test` | `127.0.0.1:55432` |
| `init` | تكتب `.env.dev` بمفتاح سري جديد في المرة الأولى، ثم تخرج | |
| `server` | تبني السيرفر من نسختك وتشغّله، وتراقب ملفات Go (تفحص كل ثانية، وهذا يعمل على Windows وmacOS أيضًا) | `http://localhost:8080` (الـ API فقط) |
| `web` | Vite مع إعادة التحميل المباشر، وتوجّه `/api` إلى `server` | `http://localhost:5173` |

الأوامر اليومية، من جذر المستودع:

| الأمر | ماذا يفعل |
|---|---|
| `docker compose -f deploy/dev/compose.yaml up -d` | يبدأ في الخلفية |
| `docker compose -f deploy/dev/compose.yaml logs -f server` | يتابع سجل السيرفر، ومنه رابط الإعداد |
| `docker compose -f deploy/dev/compose.yaml restart server` | يعيد تشغيل السيرفر، فيطبع رابط إعداد جديدًا ما دام لا يوجد صاحب نسخة بعد |
| `docker compose -f deploy/dev/compose.yaml restart web` | يعيد تثبيت اعتماديات الواجهة بعد تغيّر `package.json` أو ملف القفل (بعد `git pull` مثلًا) |
| `docker compose -f deploy/dev/compose.yaml exec server go test ./apps/server/...` | يشغّل اختبارات Go على `dhow_test` |
| `docker compose -f deploy/dev/compose.yaml exec web pnpm test` | يشغّل اختبارات الواجهة |
| `docker compose -f deploy/dev/compose.yaml down` | يوقف الحاويات ويحذفها، ويحتفظ بقاعدة البيانات |
| `docker compose -f deploy/dev/compose.yaml down --volumes` | يحذف أيضًا قاعدة البيانات وcaches الاعتماديات |

`Ctrl+C` يوقف الحزمة التي بدأت بدون `-d`.

**`.env.dev`.** تكتبه `init` مرة واحدة بالصلاحيات 600، بالشكل نفسه الذي يكتبه `make dev-env`، فيستطيع [المسار المباشر](#البدء-السريع) استخدامه أيضًا، ولا تلمس ملفًا موجودًا أبدًا. تأخذ الحزمة منه المفتاح السري وأي إعداد تضيفه بنفسك (SMTP مثلًا). أما العناوين و`DHOW_DEV=true` وروابط قاعدة البيانات فتأتي من الحزمة نفسها. احتفظ بالملف: الأسرار المحفوظة تحتاج إلى المفتاح نفسه في كل تشغيل. احذفه فقط مع قاعدة البيانات (`down --volumes`).

**الإعدادات.** اضبط هذه في الـ shell قبل `up`، أو في `deploy/dev/.env` (يقرؤه Compose من هناك، ويتجاهله Git):

| المتغير | القيمة الافتراضية | الاستخدام |
|---|---|---|
| `DHOW_DEV_WEB_PORT` | `5173` | الواجهة، والعنوان الذي يستخدمه رابط الإعداد |
| `DHOW_DEV_HTTP_PORT` | `8080` | السيرفر، لـ `curl` وعملاء الـ API |
| `DHOW_DEV_DB_PORT` | `55432` | PostgreSQL، لـ `psql` أو الأدوات المباشرة. ليس 5432، حتى لا يمنعه PostgreSQL مثبّت على الجهاز. يحتفظ `.env.dev` بمنفذ أول تشغيل |
| `DHOW_DEV_POLL_SECONDS` | `1` | كم مرة يفحص السيرفر ملفات Go |
| `DHOW_DEV_POSTGRES_IMAGE`، `DHOW_DEV_GO_IMAGE`، `DHOW_DEV_NODE_IMAGE` | `postgres:18`، `golang:1.27.3-bookworm`، `node:26-bookworm-slim` | سجل صور أو مرآة أخرى، مثل `mirror.gcr.io/library/postgres:18` |

على Linux وmacOS: `DHOW_DEV_DB_PORT=55433 docker compose -f deploy/dev/compose.yaml up`. وفي PowerShell: `$env:DHOW_DEV_DB_PORT = "55433"` أولًا.

### عندما يحدث خطأ

| العَرَض | السبب والإصلاح |
|---|---|
| `port is already allocated`، أو `ports are not available` على Windows | برنامج آخر يستخدم 55432 أو 5173 أو 8080. يعرض `docker ps` هل تحجزه حاوية قديمة، مثل `dhow-dev-postgres` من `make dev-db`. احذفها بـ `docker rm -f dhow-dev-postgres`. وإلا اختر منفذًا آخر بالمتغيرات السابقة. عندما لا تبدأ قاعدة البيانات، يعرض سجل `web` الرسالة `ENOTFOUND server`، لأن السيرفر ينتظرها. |
| لا يوجد إطار الإعداد في السجل | صاحب النسخة موجود بالفعل، فلا يوجد ما تُعدّه: سجّل الدخول على `http://localhost:5173`. وإلا شغّل `docker compose -f deploy/dev/compose.yaml logs server` وابحث عن الإطار أو عن `setup_url`. |
| `CSRF_FAILED` عند الإعداد أو الدخول | افتح التطبيق على `http://localhost:5173` (أو `DHOW_DEV_WEB_PORT` الخاص بك)، وليس على المنفذ 8080 وليس على `127.0.0.1`. |
| الواجهة تقول إن حزمة ناقصة بعد `git pull` | شغّل `docker compose -f deploy/dev/compose.yaml restart web`. |
| البناء وإعادة التحميل بطيئان على Windows | الملفات تحت `C:\` تصل إلى الحاويات عبر جسر أبطأ. لسرعة أكبر، انسخ المستودع داخل WSL2 وشغّل الأمر نفسه من shell في WSL2، مع تفعيل تكامل WSL في Docker Desktop (Settings، ثم Resources، ثم WSL integration). |
| على Linux، `apps/web/node_modules` مجلد فارغ يملكه root | أنشأه Docker كنقطة ربط لمساحة تخزين الاعتماديات. قبل أن تنتقل إلى المسار المباشر، احذفه بـ `sudo rm -rf apps/web/node_modules`. |

## سكربت الإعداد

سكربت واحد يجهّز الجهاز كله. يثبّت ما ينقص، ويسأل فقط عن الإعدادات التي تختارها أنت، ويولّد كل سر بنفسه.

**Linux وmacOS:**

```sh
git clone https://github.com/Khaymah/dhow.git
cd dhow
make dev-setup
```

**Windows**، في PowerShell من نسخة من المستودع أو من نسخة منزّلة من `deploy/dev/setup.ps1`:

```powershell
powershell -ExecutionPolicy Bypass -File deploy\dev\setup.ps1
```

على Windows، يفعل السكربت ما يلي:

1. يثبّت WSL2 مع Ubuntu 24.04 إذا كان ناقصًا. أنشئ مستخدم Linux عندما يفتح، ثم شغّل السكربت مرة أخرى.
2. ينسخ المستودع إلى `~/src/dhow` داخله. إذا كانت النسخة موجودة، يحدّثها السكربت بـ `git pull --ff-only` بدلًا من ذلك. إذا فشل ذلك بسبب تغييرات محلية أو فرع آخر، يحذّرك ويكمل بالنسخة كما هي.
3. يشغّل سكربت Linux هناك. هذا السكربت يتجاهل مسارات Windows التي يضيفها WSL إلى `PATH`، فلا يعترض طريقك Node.js المثبّت على Windows.

لحاوية PostgreSQL، فعّل تكامل WSL في Docker Desktop لـ Ubuntu 24.04: Settings، ثم Resources، ثم WSL integration.

`make` والأدوات داخل Ubuntu، وليست في PowerShell. شغّلها من shell هناك (`wsl -d Ubuntu-24.04 --cd ~`، ثم `cd src/dhow`)، أو مباشرة من PowerShell:

```powershell
wsl -d Ubuntu-24.04 --cd ~ -- bash -lc 'cd src/dhow && make dev'
```

`bash -l` يقرأ `~/.profile`، حيث وضع السكربت Go وNode.js في `PATH`.

ما يفعله السكربت، ويسأل قبل أي شيء يحتاج إلى `sudo` أو يغيّر نظامك:

| الخطوة | ماذا يحدث |
|---|---|
| الأدوات | يفحص Git وMake وcurl وPython 3 و`libatomic` (الذي يحتاجه Node.js)، ويثبّتها بـ `apt` أو `dnf` (وعلى macOS، أدوات Xcode Command Line Tools). يثبّت Go وNode.js LTS إذا كانا ناقصين أو قديمين: بـ Homebrew على macOS، وإلا من go.dev وnodejs.org إلى `~/.local/share/dhow-dev`، مع فحص كل أرشيف مقابل الـ checksum الذي ينشره موقعه. ثم pnpm بالإصدار المذكور في `apps/web/package.json`. |
| PostgreSQL 18 | `container` يشغّله في Docker أو Podman، و`native` يثبّته من مستودع مشروع PostgreSQL (Debian، وUbuntu، وFedora، وعائلة RHEL) أو Homebrew (macOS)، و`url` يستخدم قاعدة بيانات لديك. يرفض أي إصدار أقدم من 18. |
| الأسئلة | منفذ السيرفر (8080)، ومنفذ PostgreSQL (5432)، وهل ترسل البريد عبر خادم SMTP (الخادم، والمنفذ، ونوع TLS، والمستخدم، وكلمة السر مخفية أثناء الكتابة، والمرسل). |
| ما يولّده | `DHOW_SECRET_KEY` (32 بايت عشوائية)، وكلمة سر عشوائية لمستخدم قاعدة البيانات `dhow`. |
| ما يكتبه | `.env.dev`، بالصلاحيات 600، ويتجاهله Git. شغّل السكربت مرة أخرى لتغيير إعداد: يحتفظ بالمفتاح السري وكلمة سر قاعدة البيانات ما لم تطلب قيمًا جديدة. |
| في النهاية | يبني الواجهة، ويعرض إضافة الأدوات الجديدة إلى `PATH` في ملف الـ shell الخاص بك، ويطبع الأوامر التالية. |

السكربت `deploy/dev/test/fresh.sh` يشغّل السكربت كله على صورة حاوية فارغة كمستخدم جديد، مع بدائل Windows لـ `node` و`npm` و`corepack` في `PATH` كما في WSL. يشغّله CI على Ubuntu 24.04 وUbuntu 22.04 وDebian 12 وFedora:

```bash
docker run --rm -v "$PWD":/repo:ro ubuntu:24.04 /repo/deploy/dev/test/fresh.sh --yes --db=url --db-url=postgres://u:p@127.0.0.1/dhow --test-db-url=postgres://u:p@127.0.0.1/dhow_test
```

الخيارات: `--yes` يأخذ كل القيم الافتراضية بدون سؤال، و`--db=container|native|url` يختار قاعدة البيانات، و`--db-url=` و`--test-db-url=` يعطيان رابطًا، و`--port=` و`--db-port=` يضبطان المنافذ، و`--skip-web` يتخطى بناء الواجهة، و`--check` يذكر فقط الأدوات الموجودة. مع `make`، مرّرها هكذا: `make dev-setup SETUP_ARGS="--db=native"`. [البدء السريع](#البدء-السريع) التالي ينفّذ الخطوات نفسها يدويًا.

## ما تحتاجه

| الأداة | لماذا | ملاحظات |
|---|---|---|
| Git | المستودع | |
| Go | السيرفر، وجهاز التشغيل، والاختبارات | أي إصدار حديث من Go. الأمر `go` ينزّل بنفسه الإصدار الدقيق المثبّت في `go.mod`. |
| Node.js LTS مع pnpm | الواجهة (`apps/web`) | شغّل `corepack enable`، ثم `corepack prepare pnpm@10.28.0 --activate` (الإصدار في `apps/web/package.json`). |
| **PostgreSQL 18** | السيرفر واختبارات قاعدة البيانات | 18 أو أحدث بالضبط: التحديثات تستخدم `uuidv7()` المدمجة. Docker أو Podman، أو تثبيت مباشر. راجع [ما يلي](#postgresql-18). |
| Python 3 | `make check` يشغّل اختبارات hooks الخاصة بـ Claude Code | أي Python 3. |
| GNU Make وshell متوافق مع POSIX | أهداف `make` | موجودة بالفعل على Linux وmacOS، وفي WSL2. |

**Windows:** [سكربت الإعداد](#سكربت-الإعداد) يجهّز WSL2 لك. يدويًا: اعمل داخل WSL2 مع Ubuntu 24.04 أو أحدث، وانسخ المستودع إلى نظام ملفات Linux (`~/src/dhow`)، وليس تحت `/mnt/c`، فهو أبطأ بكثير. عندها يعمل كل شيء في هذه الصفحة كما على Linux، ويصل المتصفح على Windows إلى `http://localhost:8080` في WSL2.

## البدء السريع

```sh
make dev-env
make dev-db
make web
make dev
```

1. `make dev-env` يكتب `.env.dev` مرة واحدة، مع `DHOW_DEV=true`، ومفتاح سري جديد، و`DHOW_PUBLIC_URL=http://localhost:8080`، ورابطي قاعدة البيانات. يتجاهله Git. احتفظ به: الأسرار المحفوظة تحتاج إلى المفتاح نفسه في كل تشغيل. احذفه فقط مع قاعدة البيانات.
2. `make dev-db` يشغّل PostgreSQL 18 في Docker، أو Podman إذا لم يوجد Docker، على `127.0.0.1:5432`، مع قاعدتي البيانات `dhow` و`dhow_test`. تشغيله مرة أخرى آمن. بدون Docker أو Podman، ثبّت PostgreSQL مباشرة بدلًا من ذلك ([كما يلي](#postgresql-18)).
3. `make web` يبني الواجهة داخل السيرفر. بدونه يعمل السيرفر، لكن `/` يرد بـ 404. لإعادة التحميل المباشر، تخطَّه وراجع [الواجهة مع إعادة التحميل المباشر](#الواجهة-مع-إعادة-التحميل-المباشر).
4. `make dev` يشغّل السيرفر من الكود. يطبّق التحديثات، ويكتب سطر `setup_url` مثل `http://localhost:8080/setup#...`، ويستمع على `127.0.0.1:8080`.

افتح رابط الإعداد في Chrome أو Firefox خلال ساعة لتنشئ حساب صاحب النسخة. أعد تشغيل السيرفر لتحصل على رابط جديد. افحص السيرفر من طرفية أخرى بـ `curl -s http://localhost:8080/api/v1/health/ready`.

## PostgreSQL 18

اختر طريقة واحدة. كلها تنتهي بالمستخدم وكلمة السر وقواعد البيانات نفسها، فيعمل `.env.dev` مع أي منها.

### Docker أو Podman

`make dev-db` يستخدم `docker` إذا كان مثبّتًا، وإلا `podman`. اختر أحدهما بـ `make dev-db DEV_DB_ENGINE=podman`. إذا كان المنفذ 5432 مشغولًا، فاختر منفذًا آخر لقاعدة البيانات وللروابط معًا، قبل أول `make dev-env`:

```sh
make dev-env DEV_DB_PORT=5433
make dev-db DEV_DB_PORT=5433
```

`make dev-db-reset` يحذف الحاوية وبياناتها.

### التثبيت المباشر

**Debian وUbuntu (وWSL2)**، من مستودع مشروع PostgreSQL نفسه، لأن التوزيعة قد تأتي بإصدار أقدم:

```sh
sudo apt install -y postgresql-common
sudo /usr/share/postgresql-common/pgdg/apt.postgresql.org.sh -y
sudo apt install -y postgresql-18
sudo service postgresql start
```

**Fedora وRHEL وRocky وAlma:** أضف المستودع من [postgresql.org/download/linux/redhat](https://www.postgresql.org/download/linux/redhat/)، ثم ثبّت `postgresql18-server` وشغّله كما تعرض تلك الصفحة.

**macOS:** `brew install postgresql@18`، ثم `brew services start postgresql@18`. يعمل أيضًا [Postgres.app](https://postgresapp.com/) مع PostgreSQL 18.

ثم أنشئ المستخدم وقاعدتي البيانات مرة واحدة. على Linux شغّل `sudo -u postgres psql`، وعلى macOS شغّل `psql postgres`، واكتب:

```sql
CREATE ROLE dhow LOGIN PASSWORD 'dhow-dev-only';
CREATE DATABASE dhow OWNER dhow;
CREATE DATABASE dhow_test OWNER dhow;
```

تحقق بـ `psql 'postgres://dhow:dhow-dev-only@127.0.0.1:5432/dhow' -c 'SELECT version()'`. يجب أن يقول 18 أو أحدث.

### قاعدة بيانات مستضافة

أي PostgreSQL 18 يعمل إذا وضعت رابطه في `.env.dev` (مع `sslmode=require`). هذا أبطأ، ويحتاج إلى الإنترنت، والاختبارات تنشئ schema لكل اختبار وتحذفها. لذلك استخدم قاعدة بيانات خاصة لـ `DHOW_TEST_DATABASE_URL`.

## الواجهة مع إعادة التحميل المباشر

شغّل طرفيتين:

```sh
make dev-vite
```

```sh
pnpm -C apps/web install
pnpm -C apps/web dev
```

ثم افتح `http://localhost:5173`. يخدم Vite الواجهة ويوجّه `/api` إلى السيرفر على المنفذ 8080. `make dev-vite` يشغّل السيرفر مع `DHOW_PUBLIC_URL=http://localhost:5173`، لأن داو (Dhow) يقارن `Origin` من المتصفح به حرفًا بحرف. افتح رابط الإعداد على المنفذ 5173 أيضًا.

## الاختبارات والفحوص

```sh
make dev-check
```

`make dev-check` يشغّل `make check` (الـ lint، وlint الـ workflows، وفحوص بروتوكول جهاز التشغيل، وكل اختبارات Go مع كاشف السباقات، وفحص الثغرات، واختبارات الـ hooks، وفحص الوثائق) مع قاعدة بيانات الاختبار من `.env.dev`.

> [!IMPORTANT]
> استخدم `make dev-check`، وليس `make check` وحده. بدون `DHOW_TEST_DATABASE_URL` **تتخطى اختبارات قاعدة البيانات نفسها**، فقد ينجح `make check` على جهازك بينما لم تعمل أغلب اختبارات السيرفر. CI يضبطه دائمًا.

لحزمة واحدة، حمّل الملف أولًا:

```sh
set -a; . ./.env.dev; set +a
go test -race -count=1 ./apps/server/internal/Modules/Session/...
```

للتغييرات في `apps/web`، شغّل أيضًا ما يشغّله الـ CI job الخاص بها:

```sh
pnpm -C apps/web lint
pnpm -C apps/web typecheck
pnpm -C apps/web test
pnpm -C apps/web build
pnpm -C apps/web size
```

`make gen` يعيد توليد الكود من `packages/openapi/openapi.yaml` واستعلامات SQL، و`pnpm -C apps/web gen:api` يعيد توليد أنواع الـ API في الواجهة. لا تعدّل الملفات المولَّدة يدويًا أبدًا.

## جهاز التشغيل

ما تستطيع فعله يعتمد على نظامك:

| | Linux | macOS وWindows |
|---|---|---|
| اختبارات جهاز التشغيل مع المشغّل الوهمي (`make test-runner`) | ✅ | ✅ (Windows في WSL2) |
| التسجيل على السيرفر المحلي | ✅ | ✅ |
| تشغيل التشغيلات في بيئة Podman المعزولة بلا root، و`make runner-isolation` | ✅ مع المتطلبات التالية | في آلة Linux افتراضية فقط |

- **الاختبارات.** `make test-runner` يبني جهاز التشغيل مع الوسم `dhowtest`، الذي يضيف مشغّل بيئة معزولة وهميًا يعيد مخرجات مسجّلة ولا يشغّل أي عملية. أغلب العمل على جهاز التشغيل تغطيه هذه الاختبارات.
- **التسجيل.** أنشئ رمز تسجيل كما في [تسجيل جهاز تشغيل](runner/register.md)، ثم سجّل مع `--dev`، الذي يسمح بـ `http://` على `localhost` فقط، ومع مجلد حالة خاص بك:

  ```sh
  DHOW_RUNNER_STATE_DIR="$HOME/.local/state/dhow-runner-dev" \
    go run ./apps/runner/cmd/dhow-runner register --dev --url http://localhost:8080 --token-file -
  ```

- **البيئة المعزولة.** التشغيلات تعمل في Podman بلا root. يحتاج إلى Linux مع Podman 5.4 أو أحدث، و`crun`، و`uidmap`، وcgroup v2 مع تفويض المتحكمات `cpu` و`io` و`memory` و`pids` إلى جهاز التشغيل (شغّله تحت systemd مع `Delegate=yes`)، و65536 sub-id على الأقل لكل تشغيل في `/etc/subuid` و`/etc/subgid`، وuser namespaces (على Ubuntu 24.04 وأحدث، `kernel.apparmor_restrict_unprivileged_userns=0`). راجع [تثبيت جهاز تشغيل](runner/install.md). على macOS وWindows، استخدم آلة Linux افتراضية مع systemd، مثل [Lima](https://lima-vm.io/) أو Multipass على macOS، أو WSL2 مع تفعيل systemd، وشغّل جهاز التشغيل هناك مع السيرفر الذي على جهازك.
- **مجموعات اختبار العزل.** `make runner-isolation` يبني جهاز التشغيل وأداة فحص للهروب، ويشغّلهما في حاويات حقيقية بلا root، ويفشل إذا نجحت أي طريقة هروب أو لم يُفرض حد. يحتاج إلى متطلبات البيئة المعزولة السابقة و`busybox-static`. يشغّله CI على `ubuntu-26.04` كمستخدم مخصص تحت `systemd-run -p Delegate=yes` (راجع الـ job المسمى `isolation` في `.github/workflows/ci.yml`). كرّر هذا الإعداد على جهازك بـ `make runner-isolation ISOLATION_RUN="sudo systemd-run --uid=$USER --pipe --wait --collect -p Delegate=yes --"`.

## مشكلات شائعة

| العَرَض | السبب والإصلاح |
|---|---|
| تفشل التحديثات في أول تشغيل | PostgreSQL أقدم من 18 (بلا `uuidv7()`). افحص `SELECT version()`. |
| `CSRF_FAILED` عند الإعداد أو الدخول | العنوان في المتصفح ليس `DHOW_PUBLIC_URL` بالضبط: `localhost` و`127.0.0.1` مختلفان، وكذلك المنفذان 8080 و5173. |
| الدخول من جهاز آخر لا يبدو أنه يفعل شيئًا | كوكي الجلسة `Secure`، و`localhost` وحده سياق آمن عبر HTTP. استخدم الجهاز نفسه، أو HTTPS. راجع [شغّل داو على جهازك](install/local.md#المتصفحات-وhttplocalhost). |
| `make dev-db` يقول إن المنفذ مستخدم | PostgreSQL آخر يستمع على 5432. أوقفه، أو استخدم `DEV_DB_PORT` كما [سبق](#docker-أو-podman). |
| `/` يرد بـ 404 | الواجهة غير مبنية داخل السيرفر. شغّل `make web`، أو استخدم إعداد [إعادة التحميل المباشر](#الواجهة-مع-إعادة-التحميل-المباشر). |
| رابط الإعداد لم يعد يعمل | صلاحيته ساعة واحدة. أعد تشغيل السيرفر لتحصل على رابط جديد. |
| لا يمكن قراءة الأسرار المحفوظة بعد إعادة التشغيل | تغيّر `DHOW_SECRET_KEY`. احتفظ بـ `.env.dev`، أو ابدأ من جديد بقاعدة بيانات فارغة. |

## البدء من جديد

```sh
make dev-db-reset
rm .env.dev
```

ثم اتبع [البدء السريع](#البدء-السريع) مرة أخرى.
