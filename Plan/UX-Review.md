# مراجعة الواجهة وتجربة الاستخدام (UI/UX) للوحة داو

- **التاريخ:** 2026-10-28
- **النسخة:** `be0f272` على `main`، والواجهة مبنية بـ `make web` ومدمجة في السيرفر.
- **البيئة:** `DHOW_DEV=true` على `localhost:8081`، وPostgreSQL 18 في حاوية.
- **الطريقة:** Playwright على كل route وكل قيمة `?stack=` وكل تبويب وكل شريحة ممكن توصلها من الواجهة، بالعربي والإنجليزي، على 1440x900 و390x844، مع تسجيل كل طلبات الـ API لكل شاشة.
- **البيانات:** مؤسستين، و3 فرق، و3 أعضاء (owner وdeveloper وmaintainer)، و5 دعوات مستنية، وتوكن تسجيل runner، و3 مفاتيح API، و3 jobs في الطابور. وكمان دخلنا بحساب developer.
- **الصور:** في `/tmp/claude-0/ux-review/shots/` (178 صورة). الاسم بالشكل `<الشاشة>-<ar|en>-<desk|mob>.png`.
- المراجعة دي قياس ووصف بس. مفيش تعديل في كود التطبيق.

## ملخص

- **المشكلة الأساسية هيكلية مش ألوان.** كل حاجة في اللوحة شريحة عرضها 520px فوق رئيسية فاضية. على شاشة 1440 فيه حوالي 700px متعتّمة ومش مستخدمة في كل شاشة. ده اللي بيدّي إحساس «باهت وفاضي».
- **الرئيسية مش بتقول أي حاجة مفيدة.** فيها رابطين (حالة السيرفر والمظهر) وكارت مشاريع فاضي. مع إن عندنا 3 jobs واقفة في الطابور من غير runner، و5 دعوات مستنية، وصاحب المؤسسة من غير خطوة تانية، وكل ده مش باين.
- **التنقّل مربك.** «الفريق» بيفتح شريحة اسمها «المؤسسة» فيها تبويب «الفرق». و«الإعدادات» بتفتح المظهر بس. وإعدادات المؤسسة مستخبية في التبويب الرابع جوه «الفريق». و«المشاريع» و«أجهزة التشغيل» قايمتين شكلهم شغّال وهم مقفولين.
- **فيه API كامل من غير واجهة:** الـ runners وتوكنات التسجيل والـ jobs والـ logs الحية ومفاتيح API المؤسسة. ده 15 endpoint من 82.
- **الخطوط صغيرة.** 54% من النصوص بحجم 13px، وهو الحد الأدنى، والعناوين 15px زي النص. والعربي بيظهر بنفس مقاس اللاتيني، مع إن `Tokens.json` بيقول إنه لازم يبقى 1.1x.
- **عطل التبويبات اتأكد واتعرف سببه:** شريط التبويبات بيتقلّص لـ 10px لما محتوى التبويب يطول عن الشريحة (سطر 167 في `Slide.tsx` مع سطر 50 في `Tabs.tsx`).
- **الطلبات المكررة سببها اتنين:** الشريحة اللي بتتقفل بتتركّب من جديد للحركة، والـ queries عليها `staleTime: 0`. قفل شريحة «حسابي» لوحده بيبعت 6 طلبات.

## أعطال

### ع1. التبويبات بتختفي في `?stack=organization` (blocker)

- **الخطوات:** افتح الفريق، واختار «الإعدادات». على الموبايل بالإنجليزي «الدعوات» كمان.
- **اللي بيحصل:** الـ `tablist` ارتفاعه بيبقى 10px بدل 44px. بيفضل خط رفيع ومفيش طريقة ترجع للأعضاء غير إنك تقفل الشريحة.
- **الصور:** `/tmp/claude-0/ux-review/shots/11-organization-tab-settings-ar-desk.png` و`/tmp/claude-0/ux-review/shots/11-organization-tab-settings-ar-mob.png` و`/tmp/claude-0/ux-review/shots/11-organization-tab-invitations-en-mob.png`. والمقارنة مع `/tmp/claude-0/ux-review/shots/11-organization-tab-members-ar-desk.png`.
- **السبب:**
  - `apps/web/Src/Core/Ui/Slide.tsx:167`: جسم الشريحة `grid min-h-0 flex-1 content-start overflow-auto`، يعني صفوف `auto` جوه ارتفاع محدد.
  - `apps/web/Src/Core/Ui/Tabs.tsx:50`: الـ `tablist` عليه `overflow-x-auto`، فبيبقى scroll container. والـ automatic minimum size لأي scroll container بيبقى صفر.
  - لما المحتوى يطول عن الشريحة، الـ grid بيدّي الصف أقل حجم مسموح، وهو الـ padding والـ border بس (4+4+1+1 = 10px). في «الأعضاء» المحتوى قصير، فالمشكلة مش بتبان.
  - أي عنصر تاني فيه `overflow-*` جوه شريحة هيحصله نفس الشيء.
- **الإصلاح:** `min-h-fit` (أو `shrink-0` مع `flex-col`) على الـ `tablist`. والأحسن كمان إن الشريط يبقى `sticky top-0` عشان يفضل ظاهر وانت بتعمل scroll. ولازم اختبار e2e بيقيس ارتفاع الشريط في التبويب الأخير، لأن jsdom مابيحسبش layout.

### ع2. قفل أي شريحة بيعيد تحميل كل بياناتها (major)

- **القياس:** قفل «حسابي» بـ Escape بيبعت 6 طلبات (mfa وpasskeys وoidc/providers وidentities وapi-tokens وsessions). وقفل «سجل الأحداث» بيعيد `audit-events`. و`GET /organizations/{id}` اتطلب 5 مرات في دقيقة تنقّل واحدة.
- **السبب:**
  - `apps/web/Src/Core/Ui/SlideStack.tsx:194`: الشريحة اللي بتخرج بتترسم من `ghosts.map` في شجرة React جديدة، فكل مكوّناتها بتتركّب من الأول.
  - الـ queries دي عليها `staleTime: 0`، فأي تركيب جديد يعني طلب جديد: `Core/Auth/useMfaStatus.ts:8` و`Modules/Organization/Hooks/useOrganization.ts:28` و`:32` و`:38` و`:45` و`Modules/Session/Hooks/useSessions.ts:14` و`Modules/Mfa/Hooks/useMfa.ts:12` و`Modules/Audit/Hooks/useAuditEvents.ts:19`.
- **ملاحظة:** إصلاح الـ caching شغّال في branch تاني، فهنا قسنا بس. الشبح محتاج يعرض لقطة ثابتة أو يعيد استخدام نفس العنصر، مش يركّب الجسم من الأول.

### ع3. `/me/mfa` مرتين في فتح «حسابي» (minor)

- `MfaSection` و`SecondFactorCard` و`useMfaStatus` بيستخدموا نفس الـ query بـ `staleTime: 0` (`Core/Auth/useMfaStatus.ts:8`). كل مكوّن بيتحمّل في chunk لوحده، فالتاني بيتركّب بعد الأول ما يخلص، ويبعت طلب تاني.

### ع4. الأعضاء بيتجابوا مرتين بمفاتيح مختلفة (minor)

- `Modules/Organization/Components/MembersCard.tsx:64` بيجيب صفحة بـ `PAGE_SIZE`، و`MemberBody.tsx:96` و`TransferBody.tsx:28` بيجيبوا كل الأعضاء عشان يلاقوا عضو واحد. مفيش `GET /organizations/{id}/members/{userId}` في الـ API.
- **الإصلاح:** endpoint للعضو الواحد، أو مفتاح واحد للقايمة الكاملة يستخدمه الكل.

### ع5. أحداث الـ runners والـ jobs بتظهر كمفاتيح خام في سجل الأحداث (major)

- **الصورة:** `/tmp/claude-0/ux-review/shots/10-audit-ar-desk.png`، فيها `job.created` بخط mono إنجليزي وسط العربي.
- **السبب:** `Modules/Audit/Types/AuditActions.ts:10-34` مافيهاش أي action لـ `job.*` أو `runner.*`، ومافيش مفاتيح `audit.action.*` ليهم في `I18n/Locales/Workspace/`. فـ `AuditEventRow.tsx:20` بيعرض الاسم الخام.
- **الناقص:** `job.created` و`job.cancel_requested` و`runner.registered` و`runner.registration_token_created` و`runner.registration_token_revoked` و`runner.keys_rotated` و`runner.disabled_clone_suspected`.
- **الإصلاح:** المفاتيح بالعربي والإنجليزي والأيقونات، واختبار بيقارن قايمة الـ actions في السيرفر بالكتالوج.

### ع6. فتح شريحة TOTP بيعمل secret جديد كل مرة (minor)

- `Modules/Mfa/Components/TotpBody.tsx:96-98` بيبعت `POST /me/totp` أول ما الشريحة تتركّب. أي refresh أو رابط `?stack=account,account-totp` بيعمل secret جديد ويلغي الـ QR اللي اتمسح.
- **الإصلاح:** يبدأ التسجيل بزرار «ابدأ»، أو يحتفظ بالتسجيل المفتوح لحد ما يخلص.

### ع7. عناوين شرائح العضو والفريق عامة (minor)

- `App/Layout/OrganizationSlides.tsx:56` و`:65`: العنوان «الأعضاء» و«الفرق» بدل اسم العضو أو الفريق. والـ breadcrumb بيبقى «المؤسسة › الفرق» ومش بيقول أنهي فريق.
- **الصور:** `/tmp/claude-0/ux-review/shots/10-team-detail-ar-desk.png` و`/tmp/claude-0/ux-review/shots/10-member-other-ar-desk.png`.

### ع8. نص TOTP بيقول «مفاتيح المرور كفاية» وانت معندكش ولا مفتاح (minor)

- `Modules/Mfa/Components/FactorsCard.tsx:23` بيستخدم `mfa.totp.off` في كل الحالات. لازم نص تاني لما `PasskeyCount` يبقى صفر.
- **الصورة:** `/tmp/claude-0/ux-review/shots/10-account-ar-desk.png`.

### ع9. اللغة بتتقلب بعد التحميل (minor)

- لو اللغة في `localStorage` غير المحفوظة على السيرفر، الصفحة بتترسم بلغة وبعدين بتتقلب (`Core/Preferences/usePreferencesSync.ts:48-50`). فتح الرابط بالإنجليزي على جهاز جديد بيرجع عربي لوحده.

## صفحات ناقصة

مقارنة `packages/openapi/openapi.yaml` (82 endpoint) بالواجهة:

| الصفحة | الـ API | الواجهة دلوقتي | الأولوية |
|---|---|---|---|
| أجهزة التشغيل: قايمة وتفاصيل وتغيير اسم وإلغاء | `GET/PATCH /organizations/{id}/runners[/{runnerId}]` و`POST .../revoke` | عنصر في القايمة مقفول «جاية قريب» (`App/Layout/ShellMenu.tsx:27`) | عالية (M3.12) |
| توكن تسجيل جهاز تشغيل وأمر التسجيل | `GET/POST /runner-registration-tokens` و`revoke` | مفيش. الـ `RegisterCommand` مابيظهرش في أي حتة | عالية (M3.12) |
| التشغيلات (jobs): قايمة وتفاصيل وإلغاء | `GET/POST /jobs` و`GET /jobs/{jobId}` و`cancel` | مفيش، ولا حتى عنصر في القايمة. الـ 3 jobs اللي عملناها `NO_MATCHING_RUNNER` ومحدش يعرف | عالية (M3.12) |
| سجل التشغيل الحي | `GET /jobs/{jobId}/logs` و`/logs/stream` (SSE) | مفيش | عالية (M3.12) |
| مفاتيح API المؤسسة (للأدمن) | `GET /organizations/{id}/api-tokens` و`revoke` | بيظهر بس «مفاتيحي» في حسابي | متوسطة |
| المشاريع | مفيش API لسه (M4) | عنصر مقفول في القايمة، وكارت فاضي في الرئيسية | نخبّيه لحد M4 |
| الإشعارات | مفيش API | الجرس بيفتح كارت ثابت «مفيش إشعارات جديدة» (`App/Layout/FrameSlides.tsx:60-68`) | نخبّيه أو نربطه بأحداث حقيقية |
| الإعدادات | `PATCH /organizations/{id}` و`second-factor-rule` موجودين | «الإعدادات» بتفتح المظهر بس، وإعدادات المؤسسة في التبويب الرابع جوه «الفريق» | عالية |
| إعدادات النسخة (SMTP وOIDC وإنشاء المؤسسات) | مفيش، كلها env | مفيش ولا عرض للقراءة. حالة السيرفر بتعرض النسخة والفحوصات بس | متوسطة |
| تعديل الاسم والإيميل | مفيش `PATCH /me` | الاسم والإيميل للقراءة بس | متوسطة (API + واجهة) |
| فلترة سجل الأحداث وتفاصيل الحدث | الـ API فيه `Page` و`Limit` بس | قايمة بصفحات، والصف مش بيفتح تفاصيل (مين اتعمل عليه إيه، والـ metadata) | متوسطة |
| حذف مؤسسة | مفيش (اتأجل) | مفيش | منخفضة |

### طرق بتقف في النص

- **عضو = انت:** شريحة العضو بتقول «غيّر حسابك من حسابي» ومفيش زرار يفتحها (`/tmp/claude-0/ux-review/shots/10-member-self-ar-desk.png`).
- **صاحب المؤسسة من غير خطوة تانية:** كارت «الخطوة التانية في المؤسسة» مقفول وبيقول «فعّل في حسابك الأول»، ومفيش زرار يوديك (`/tmp/claude-0/ux-review/shots/32-organization-settings-scrolled-ar-desk.png`).
- **قبول الدعوة:** بعد ما الحساب يتعمل بيقولك «ادخل وظبّط التحقق»، وبعدين صفحة الدخول بتقول «أهلًا بيك تاني» لواحد لسه داخل أول مرة (`/tmp/claude-0/ux-review/shots/02-invite-accepted-ar-desk.png` و`/tmp/claude-0/ux-review/shots/03-login-ar-desk.png`).
- **الـ jobs:** ممكن تتعمل من الـ API، بس مفيش أي مكان في الواجهة يقولك إنها واقفة.
- **فريق برقم غلط:** «مقدرناش نلاقي الفريق ده» من غير زرار يرجّعك للفرق (`/tmp/claude-0/ux-review/shots/32-team-wrong-id-ar-desk.png`).

## ملاحظات التصميم لكل شاشة

مقياس الخطورة: **blocker** بيمنع الاستخدام، **major** بيضايق كل يوم، **minor** تحسين.

### الهيكل العام (كل الشاشات)

| # | الملاحظة | ليه مهمة | الإصلاح | الخطورة |
|---|---|---|---|---|
| ت1 | كل الأقسام شرائح 520px فوق رئيسية متعتّمة. 700px من 1440 ميتة (`/tmp/claude-0/ux-review/shots/10-account-ar-desk.png`، `/tmp/claude-0/ux-review/shots/10-audit-ar-desk.png`) | ده سبب «فاضي وباهت». القوايم الطويلة (الأعضاء والسجل) محشورة والشاشة فاضية | الأقسام الأساسية (الفريق، سجل الأحداث، أجهزة التشغيل، التشغيلات، الإعدادات) تبقى صفحات كاملة في `main` بـ route لكل واحدة. والشرائح تفضل للتفاصيل والإنشاء والتأكيد بس. ده محتاج ADR يعدّل ADR-0032/0034 | major |
| ت2 | «الفريق» بيفتح «المؤسسة» وفيها «الفرق». ثلاث كلمات لنفس المكان | المستخدم مش عارف هو فين | اسم القسم «المؤسسة» أو «الأعضاء والفرق»، والعنوان نفس الاسم | major |
| ت3 | «الإعدادات» = المظهر بس. وإعدادات المؤسسة في تبويب جوه «الفريق» | اللي بيدوّر على الإعدادات مش هيلاقيها | صفحة إعدادات فيها: المؤسسة، والأمان (الخطوة التانية)، ومفاتيح API المؤسسة، والمظهر | major |
| ت4 | «المشاريع» و«أجهزة التشغيل» بنفس لون العناصر الشغّالة تقريبًا، والتلميح «جاية قريب» بيغطي «الرئيسية» (`/tmp/claude-0/ux-review/shots/13-nav-soon-hover-ar-desk.png`) | بيبان إن الضغط مش شغّال | تتشال من القايمة لحد ما تخلص، أو تبقى `text-muted` مع شارة «قريب» جنب الاسم | major |
| ت5 | شارة «1 من 1» في كل شريحة | ضوضاء، ومعناها مش واضح | تظهر بس لما يبقى فيه أكتر من شريحة، أو تتشال ويكفي الـ breadcrumb | minor |
| ت6 | كلمة «شريحة» ظاهرة للمستخدم في البحث (`palette.kind.panel`) وفي التنبيهات | مصطلح داخلي | «صفحة» أو مفيش نوع خالص | minor |
| ت7 | مكان «إلغاء» في الـ footer مختلف: في TOTP على الناحية التانية، وفي الدعوة جنب الزرار الأساسي، وفي المفتاح الجديد الزرار في الجسم مش الـ footer (`Modules/Tokens/Components/TokenNewBody.tsx:110`) | عدم اتساق بيخلّي العين تدوّر | قاعدة واحدة: الأساسي في الـ end، والإلغاء جنبه ghost، والخطر في الـ start | minor |
| ت8 | الـ `main` محدود بـ `max-w-[760px]` (`Modules/System/Pages/WorkspacePage.tsx:20`) وبيسيب 440px فاضية على 1440 | بيكبّر الإحساس بالفراغ | grid بعمودين على الشاشات العريضة | minor |

### الخطوط والمسافات

- **ت9 (major):** توزيع المقاسات في 9 شاشات: 13px = 192 نص، و14px = 128، و15px = 23، و20px = 10. يعني أغلب النص في الحد الأدنى. وعنوان الكارت (`title-sm`) 15px زي النص العادي، فالهرمية ضعيفة. **الإصلاح:** النص الأساسي 15px والثانوي 14px، و13px للشارات بس. وعناوين الكروت 16-17px، وعنوان الصفحة `title-2xl` (26px) اللي موجود ومش مستخدم غير في صفحات الدخول.
- **ت10 (major):** `Tokens.json` بيقول «Arabic runs about 1.1× the Latin size»، بس مفيش أي قاعدة `:lang(ar)` بتطبّقها. و`--font-arabic` متعرّف ومش مستخدم في أي حتة. والـ `font-sans` بيبدأ بـ Poppins، فالأرقام والتواريخ جوه الجمل العربي بتطلع بخط هندسي عريض جنب IBM Plex Arabic (`2026/10/28، 9:21 م`). **الإصلاح:** `[lang=ar] { font-family: var(--font-arabic) }` ومقاسات عربي أكبر بـ 1-2px.
- **ت11 (minor):** الإيميلات بخط mono في كل حتة (الأعضاء والحساب). الـ mono مكانه القيم التقنية زي الهاش والتوكن (ADR-0034)، مش الإيميل.
- **ت12 (minor):** الأسطح متقاربة جدًا (`canvas #071318` و`surface #10222a` و`raised #163039`)، والخط الرفيع `#ffffff14`. الكروت بتذوب في الخلفية. **الإصلاح:** نزوّد الفرق بين الخلفية والكارت درجة، أو نخلّي الـ canvas أغمق، من غير shadow.

### الرئيسية

`/tmp/claude-0/ux-review/shots/10-home-ar-desk.png` و`/tmp/claude-0/ux-review/shots/10-home-en-desk.png` و`/tmp/claude-0/ux-review/shots/10-home-ar-mob.png`

- **ت13 (blocker):** مفيش أي معلومة عن الشغل. **المقترح:** صف إحصائيات (`text-stat` موجود ومش مستخدم): التشغيلات الشغّالة والواقفة في الطابور، وأجهزة التشغيل المتصلة، والدعوات المستنية، وحالة السيرفر كنقطة لون. وتحتهم «آخر التشغيلات» و«محتاج انتباهك» (jobs من غير runner، أعضاء من غير خطوة تانية، توكن تسجيل هيخلص).
- **ت14 (major):** لو فحص في السيرفر فشل، الرئيسية مش بتقول حاجة. البانر بيظهر بس لو `/meta` وقع (`/tmp/claude-0/ux-review/shots/31-degraded-home-ar-desk.png` مقابل `/tmp/claude-0/ux-review/shots/31-degraded-system-status-ar-desk.png`). **الإصلاح:** بانر من `/health/ready` في الرئيسية.
- **ت15 (minor):** «المظهر والألوان» في «ابدأ من هنا» مش أول حاجة صاحب نسخة جديدة محتاجها. **الأحسن:** قايمة بداية فيها: فعّل الخطوة التانية، وادعي أول عضو، وسجّل أول جهاز تشغيل، وكل واحدة بتتعلّم لما تخلص.

### المؤسسة / الفريق

`/tmp/claude-0/ux-review/shots/11-organization-tab-members-ar-desk.png` و`/tmp/claude-0/ux-review/shots/11-organization-tab-teams-ar-mob.png`

- **ت16 (blocker):** عطل التبويبات (ع1).
- **ت17 (major):** شارة «محتاج خطوة تانية» بلون كهرماني على كل الأعضاء، حتى في مؤسسة سياستها «مقفولة» (`/tmp/claude-0/ux-review/shots/20-second-org-team-ar-desk.png`). الإنذار اللي بيتكرر بيفقد معناه. **الإصلاح:** تظهر بس لما السياسة تطلبها، وغير كده شارة محايدة.
- **ت18 (minor):** صف الفريق بيقول «0 أعضاء» و«من غير دور» بنفس الوزن. الفريق الفاضي محتاج زرار «ضيف أعضاء».
- **ت19 (minor):** تبويب الدعوات بيكرر «New link» و«Revoke» تحت كل دعوة بيبقى طويل على الموبايل (`/tmp/claude-0/ux-review/shots/11-organization-tab-invitations-en-mob.png`). **الأحسن:** قايمة أفعال في الصف.

### العضو والفريق (تفاصيل)

`/tmp/claude-0/ux-review/shots/10-member-other-ar-desk.png` و`/tmp/claude-0/ux-review/shots/10-team-detail-ar-desk.png`

- **ت20 (minor):** العنوان عام (ع7).
- **ت21 (minor):** اختيار الدور 5 كروت كبيرة لكل دور، والزرار «غيّر الدور» تحتهم. في شريحة الفريق الأعضاء نفسهم تحت الـ scroll. **الأحسن:** الأعضاء الأول، والدور كـ select بوصف قصير.

### حسابي

`/tmp/claude-0/ux-review/shots/10-account-ar-desk.png` و`/tmp/claude-0/ux-review/shots/10-account-ar-mob.png` و`/tmp/claude-0/ux-review/shots/32-account-scrolled-bottom-ar-desk.png`

- **ت22 (major):** شريحة واحدة فيها 7 كروت (بياناتك، والتحقق، ومفاتيح المرور، والحسابات المربوطة، ومفاتيح API، والأجهزة). طويلة ومفيش طريقة توصل لجزء معيّن. **الإصلاح:** صفحة حساب بتبويبات: الملف، والأمان، ومفاتيح API، والأجهزة.
- **ت23 (major):** الملخص الأخضر بيقول «إنت داخل بحسابك» والتحقق بخطوتين مقفول. أهم معلومة (حسابك مش محمي) تحت. **الإصلاح:** الملخص يعكس الأمان («فعّل خطوة تانية»)، زي ما ADR-0034 بيقول.
- **ت24 (minor):** نص TOTP (ع8).
- **ت25 (minor):** «اللغة» بتظهر للقراءة بس، والتغيير من زرار في الـ top bar.

### الخطوة التانية ومفتاح API جديد

`/tmp/claude-0/ux-review/shots/10-account-totp-ar-desk.png` و`/tmp/claude-0/ux-review/shots/10-account-token-new-ar-desk.png` و`/tmp/claude-0/ux-review/shots/32-token-created-ar-desk.png`

- **ت26 (minor):** الـ QR بيبدأ secret من غير ما تطلب (ع6).
- **ت27 (minor):** المفتاح الجديد: 9 صلاحيات ككروت كبيرة، والزرار في آخر الـ scroll. **الأحسن:** checkbox list مضغوطة متجمّعة (المؤسسة، والأعضاء، وأجهزة التشغيل، والتشغيلات)، والزرار في الـ footer.
- **ت28 (minor):** شاشة «اتعمل المفتاح» كويسة (تحذير ونسخ). بس «نسخته، اقفل» زرار ghost جنب «انسخ المفتاح»، والأحسن يبقى في الـ footer.

### سجل الأحداث

`/tmp/claude-0/ux-review/shots/10-audit-ar-desk.png`

- **ت29 (major):** المفاتيح الخام (ع5).
- **ت30 (major):** كل الصفوف بنفس الأيقونة (ملف) ونفس الشكل، فالأحداث الخطرة مش باينة. ومفيش فلترة بالنوع ولا بالشخص، ومفيش تفاصيل للحدث. **الإصلاح:** جدول في صفحة كاملة، أيقونة ولون لكل فئة، وفلاتر (محتاجة API).
- **ت31 (minor):** «السلسلة لسه ماتفحصتش» بانر أزرق فوق كل حاجة. ممكن يبقى سطر صغير جنب زرار «اتحقق».

### المظهر والألوان ومحرّر الثيم

`/tmp/claude-0/ux-review/shots/10-appearance-ar-desk.png` و`/tmp/claude-0/ux-review/shots/10-theme-edit-ar-desk.png`

- **ت32 (minor):** معاينة الثيم مستطيلات فاضية. معاينة صغيرة لكارت وزرار حقيقي هتبقى أوضح.
- **ت33 (minor):** خانة «ولّد كل الخلفيات من لون واحد» مربع فاضي مش واضح إنه checkbox.

### حالة السيرفر وتفاصيل الفحص

`/tmp/claude-0/ux-review/shots/10-system-status-ar-desk.png` و`/tmp/claude-0/ux-review/shots/10-check-database-ar-desk.png`

- **ت34 (minor):** منظّمة وواضحة، ودي أحسن شاشة. بس الـ commit هاش كامل 40 حرف. **الأحسن:** 7 حروف مع زرار نسخ.
- **ت35 (minor):** مفيش وقت آخر فحص، ومفيش تحديث تلقائي.

### الدخول والإعداد والدعوة

`/tmp/claude-0/ux-review/shots/01-setup-form-ar-desk.png` و`/tmp/claude-0/ux-review/shots/03-login-ar-mob.png` و`/tmp/claude-0/ux-review/shots/02-invite-page-en-mob.png`

- **ت36 (minor):** الشاشات دي مرتّبة ومقبولة. الإعداد الأولاني فورم طويلة في عمود واحد. ممكن تتقسم خطوتين (حسابك، والمؤسسة).
- **ت37 (minor):** «أهلًا بيك تاني» حتى لأول دخول.

### الحالات (تحميل، خطأ، فاضي)

`/tmp/claude-0/ux-review/shots/30-loading-organization-ar-desk.png` و`/tmp/claude-0/ux-review/shots/31-error-account-ar-desk.png` و`/tmp/claude-0/ux-review/shots/10-notifications-ar-desk.png` و`/tmp/claude-0/ux-review/shots/10-not-found-ar-desk.png`

- **ت38 (minor):** التحميل 5 خطوط رمادية بأطوال متدرّجة، مش شبه الشكل اللي جاي. skeleton على شكل صفوف القايمة هيبقى أهدى.
- **ت39 (minor):** في الخطأ، «جرّب تاني» زرار لوحده تحت الإنذار، والـ skeleton لسه ظاهر جنبه. **الأحسن:** الزرار جوه الإنذار.
- **ت40 (minor):** صفحة 404 أيقونتها «طبقات» (نفس المشاريع). أيقونة بوصلة أو علامة استفهام أوضح.

### البحث السريع (Ctrl K)

`/tmp/claude-0/ux-review/shots/12-command-palette-ar-desk.png`

- **ت41 (minor):** 7 أوامر بس. ناقص: سجل الأحداث، وادعي عضو، وفريق جديد، ومفتاح API جديد، وبدّل المؤسسة، وغيّر اللغة، واخرج. والأعضاء والفرق نفسهم مش بيتبحث فيهم.

### الموبايل وRTL

- **ت42 (minor):** الشريط اللي تحت فيه 7 أيقونات من غير أسماء، منهم 2 مقفولين (`/tmp/claude-0/ux-review/shots/10-home-ar-mob.png`). **الأحسن:** 4-5 بأسماء، والباقي في «المزيد».
- **ت43:** الـ RTL سليم في كل الشاشات: الاتجاهات والأسهم والـ breadcrumb والشرائح بتطلع من الـ start. مفيش مشكلة اتجاه اتلاحظت.
- **ت44:** التباين كويس: النص الثالث `#93a5aa` على الكارت حوالي 6.4:1. الاستثناء الزرار المقفول (`#6b7e83` على `#163039` حوالي 3.2:1) زي «احفظ الاسم».

## جدول الطلبات

### تحميل كامل لكل شاشة (عربي، 1440)

الأساس في كل تحميل 3 طلبات: `GET /me` و`GET /meta` و`GET /me/preferences`.

| الشاشة | العدد | الطلبات غير الأساس | مكرر؟ |
|---|---|---|---|
| `/` | 3 | — | لا |
| `system-status` و`check-*` | 4 | `GET /health/ready` | لا |
| `notifications`، `appearance`، `theme-edit`، `organizations`، `organization-new` | 3 | — | لا |
| `account` | **10** | `GET /me/mfa` ×2، `/me/passkeys`، `/oidc/providers`، `/me/identities`، `/me/api-tokens`، `/sessions` | **نعم: `/me/mfa` ×2** |
| `account,account-totp` | **10** | زي حسابي + `POST /me/totp` | كتابة عند الفتح |
| `account,account-password` | 9 | زي حسابي | لا |
| `account,account-token-new` | 9 | زي حسابي | لا |
| `audit` | 5 | `GET /organizations/{id}/audit-events`، `/members` | لا |
| `organization` | 5 | `GET /organizations/{id}`، `/members` | لا |
| `organization,organization-invite` | 5 | زي المؤسسة | لا |
| `organization,team-new` | 5 | زي المؤسسة | لا |
| `organization,team-{id}` | 6 | + `GET /teams/{teamId}` | لا |
| `organization,member-{self}` | 6 | `GET /organizations/{id}`، `/members` ×2 | **نعم** |
| `organization,member-{other}` | 7 | `GET /organizations/{id}`، `/members` ×2، `/me/mfa` | **نعم** |
| `organization,organization-transfer` | 7 | `GET /organizations/{id}`، `/members` ×2، `/me/mfa` | **نعم** |
| `/login` (من غير دخول) | 2 | `GET /me` (401)، `GET /meta` | لا |

### التنقّل جوه التطبيق (نفس الصفحة)

| الخطوة | العدد | الطلبات |
|---|---|---|
| ضغط «الفريق» | 2 | `GET /organizations/{id}`، `/members` |
| تبويب الفرق / الدعوات / الإعدادات | 1 لكل واحد | `/teams`، `/invitations`، `/me/mfa` |
| الرجوع لتبويب الفرق تاني | 1 | `/teams` (`staleTime: 0`) |
| فتح عضو | 2 | `/members` (مفتاح تاني)، `/me/mfa` |
| الرجوع من العضو | 1 | `/me/mfa` |
| **Escape (قفل المؤسسة)** | 1-2 | `GET /organizations/{id}`، `/members` |
| ضغط «حسابي» | 6 | mfa، passkeys، oidc/providers، identities، api-tokens، sessions |
| **Escape (قفل حسابي)** | **6** | **نفس الـ 6 تاني** |
| ضغط «حسابي» تاني بعدها بثواني | 3 | mfa، passkeys، sessions |
| Escape | 3 | نفس الـ 3 |
| ضغط «سجل الأحداث» | 2 | `audit-events`، `/members` |
| ضغط «الرئيسية» (قفل السجل) | 1 | `audit-events` |
| ضغط اسم المؤسسة (المبدّل) | 1 | `GET /organizations/{id}` |
| تبديل المؤسسة | 1 | `PUT /me/preferences` |
| 20 ثانية من غير حركة، أو الرجوع للتبويب | 0 | مفيش polling ولا refetch on focus |

**المجموع في جولة واحدة (حوالي دقيقة):** `GET /me/mfa` 7 مرات، و`GET /organizations/{id}` 5 مرات، و`GET /sessions` و`/me/passkeys` 5 مرات لكل واحد.

## ترتيب الإصلاحات

1. **ع1 التبويبات** (blocker): `min-h-fit` و`sticky` على الـ `tablist`، واختبار e2e. ساعة شغل.
2. **ع2 وع3 وع4 الطلبات** (major): الـ caching شغّال في branch تاني. ونضيف عليه: الشبح ما يركّبش الجسم من جديد (`SlideStack.tsx:194`)، ومفتاح أعضاء واحد.
3. **ع5 أسماء أحداث السجل** (major): مفاتيح الـ `job.*` و`runner.*` واختبار تطابق.
4. **ت13 وت14 الرئيسية** (blocker): إحصائيات وحالة السيرفر و«محتاج انتباهك». أول حاجة Sabry بيشوفها.
5. **صفحات M3.12**: أجهزة التشغيل، وتوكن التسجيل بأمر التسجيل، والتشغيلات، والـ log الحي. الـ API جاهز.
6. **ت1 وت2 وت3 التنقّل** (major): ADR جديد: الأقسام صفحات كاملة، والشرائح للتفاصيل. واسم «المؤسسة» واحد، وصفحة «الإعدادات» حقيقية.
7. **ت9 وت10 الخطوط** (major): `--font-arabic` للعربي، ومقاسات أكبر، وهرمية عناوين أوضح.
8. **ت4 القايمة**: نخبّي «المشاريع» و«الإشعارات» لحد ما يبقى ليهم محتوى.
9. **ت22 وت23 الحساب**: تبويبات، وملخص بيعكس الأمان.
10. **ت17 شارة الخطوة التانية**، و**ت30 السجل** (أيقونات وفلاتر، محتاجة API).
11. مفاتيح API المؤسسة في الواجهة.
12. باقي الـ minor: ع6 وع7 وع8 وع9، وت5 وت6 وت7، والحالات، والبحث السريع، وشريط الموبايل.
