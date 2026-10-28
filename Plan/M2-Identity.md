# خطة M2: الهوية والصلاحيات

**الحالة:** اتنفّذت (M2.1 لحد M2.5)، والقرارات في [ADR-0033](ADR/0033-browser-sessions-invites-breached-passwords.md) و[ADR-0035](ADR/0035-identity-core-implementation.md) و[ADR-0037](ADR/0037-mfa-implementation.md) و[ADR-0038](ADR/0038-orgs-teams-invites-implementation.md) و[ADR-0039](ADR/0039-oidc-api-tokens-preferences.md) و[ADR-0040](ADR/0040-identity-hardening.md) · **التاريخ:** 2026-10-27، وآخر تحديث 2026-10-28

## الهدف
شرط خلاص M2 في `docs/roadmap.md`:
- كل اختبارات الصلاحيات خضرا.
- مراجعة داخلية لمسارات المصادقة من غير finding مفتوح Critical أو High ([قايمة الفحص](../docs/security/auth-review-checklist.md)).
- **قرار Sabry (ADR-0040):** المراجعة الخارجية اتنقلت لـ M9 مع الـ penetration test.

المراجع:
- [ADR-0013](ADR/0013-authentication-methods.md) و[ADR-0015](ADR/0015-rbac-model.md) و[ADR-0016](ADR/0016-multi-org.md).
- `docs/security-model.md`، الأقسام من 3 لـ 6، والقسم 9.

## اللي هيتبني

### 1. الإعداد الأولاني (Owner setup)
- التسجيل العام مقفول.
- أول ما السيرفر يقوم من غير owner، بيطبع في الـ log رابط إعداد بيتستخدم مرة واحدة: token عشوائي 32 byte، متخزّن hashed، وصلاحيته ساعة.
- الرابط ده بيعمل أول owner وأول organization، وبعدها بيتلغي.
- `SetupRequired` في `/meta` بيبقى false.

### 2. كلمة السر
- Argon2id (`golang.org/x/crypto/argon2`)، بمعاملات OWASP.
- الطول من 12 لـ 128 حرف.
- فحص كلمات السر المسرّبة بـ k-anonymity (Have I Been Pwned)، اختياري.
- Lockout لكل حساب ولكل IP بـ exponential backoff.
- الرسايل عامة، ووقت الرد ثابت.

### 3. التحقق بخطوتين
- **TOTP:**
  - تفعيله أو تغييره محتاج الـ factor الحالي.
  - حماية من الـ replay: آخر step اتقبل بيتخزّن.
  - recovery codes متخزّنة hashed.
- **Passkeys (WebAuthn):** الطريقة الموصى بيها، وبتدعم تسجيل دخول من غير كلمة سر.
- **خطوة كلمة السر:** بتطلّع challenge اسمه `mfa_pending`. الـ challenge ده قصير العمر، وبيتستخدم مرة واحدة، ومربوط بالمستخدم والجهاز.

### 4. OIDC
- تسجيل دخول بـ GitHub وGoogle وأي IdP بيدعم OIDC، بـ PKCE وstate وnonce.
- ربط الحساب بيحصل بالـ `sub` بس، مش بالإيميل.
- قايمة دومينات مسموحة اختيارية.

### 5. الجلسات (قرار مطلوب، تحت)
- كل جلسة ليها صف في `"Sessions"`.
- rotation عند تسجيل الدخول وعند أي تغيير في الصلاحيات.
- إلغاء شامل عند: تسجيل الخروج، وتغيير كلمة السر، وتغيير الـ MFA، وتغيير الدور، و«اخرج من كل الأجهزة».
- حماية من CSRF.

### 6. المؤسسات والفرق والأدوار
- **الجداول:** `"Organizations"` و`"Users"` و`"Memberships"` و`"Teams"` و`"TeamMembers"` و`"RoleBindings"`. الـ scope بيبقى org أو project أو environment.
- **الأدوار الثابتة:** Owner وAdmin وMaintainer وDeveloper وViewer.
- **`can(subject, action, resource)`** في `Core/Policy`:
  - كل route بيعلن الـ action والـ resource بتوعه.
  - الـ middleware بتطبّق `can()`.
  - أي loader محصور في نطاق الـ tenant، فالـ check الناقص بيرجّع 404 مش تسريب.
- **اختبار الـ route-table:** بيلف على كل route مع كل دور وكل scope. أي route من غير policy معلنة بيوقّع الـ build.
- **hook جديد:** أي route في `*.Routes.go` من غير policy بيترفض.

### 7. الدعوات وAPI tokens
- **الدعوات:** بتتعمل لإيميل ودور، وبتتستخدم مرة واحدة، ولها مدة صلاحية، وبتتخزّن hashed (قرار الإيميل تحت).
- **API tokens:**
  - بتبدأ بـ `dhow_pat_`، ومتخزّنة HMAC-SHA256 بـ pepper من `DHOW_SECRET_KEY`.
  - ليها scope وصلاحيات صريحة وتاريخ انتهاء إجباري.
  - عمر الـ token ما يزيدش عن صلاحيات اللي عمله.

### 8. الـ Audit log
- `"AuditEvents"` جدول append-only، وكل event فيه `"PrevHash"`، فأي تلاعب بيبان.
- **بيتسجّل:**
  - أحداث المصادقة، وتغييرات الأدوار، والدعوات، والـ tokens، والجلسات.
  - كل ده بيعدّي على `Core/EventBus`.
- الاحتفاظ 90 يوم.
- endpoint للتحقق من السلسلة.

### 9. الواجهة
- **صفحات مستقلة بره الـ shell:** الإعداد الأولاني، وتسجيل الدخول، والتحقق بخطوتين، وقبول الدعوة.
- **شرائح جوه الـ shell:**
  - حسابي: كلمة السر، والـ passkeys، والـ TOTP، والجلسات، والـ tokens.
  - المؤسسة: الأعضاء، والفرق، والأدوار، والدعوات.
  - سجل الأحداث.
- الثيم والتفضيلات بيتنقلوا من `localStorage` لتفضيلات المستخدم في السيرفر.

## تقسيم الـ PRs
| PR | المحتوى |
|---|---|
| M2.1 | الجداول الأساسية، والإعداد الأولاني، وكلمة السر، والجلسات، وتسجيل الدخول والخروج، و`Core/Policy` بـ `can()`، واختبار الـ route-table، والـ audit log |
| M2.2 | TOTP وrecovery codes وPasskeys، وخطوة `mfa_pending` |
| M2.3 | المؤسسات والفرق والأدوار والدعوات، وشرائح المؤسسة |
| M2.4 | OIDC، وAPI tokens، وتفضيلات المستخدم (الثيم) |
| M2.5 | تقوية: rate limiter عام، وإصلاح الـ audit sanitizer، ومسح الصفوف المنتهية، وfuzz لمدخلات المصادقة، واختبارات توقيت، والمراجعة الداخلية وقايمة فحص للمراجعة الخارجية ([الخطة](M2-Hardening.md)) |

كل PR بيعدّي من `make check` و`make smoke` واختبارات الواجهة، وفيه اختبارات سلبية لكل مسار أمني.

## الاعتماديات المتوقعة (بنسخ مثبّتة)
- `golang.org/x/crypto`: argon2، وهي موجودة أصلًا عن طريق validator.
- `github.com/go-webauthn/webauthn`: الـ passkeys، ومفيش بديل معقول يتكتب بإيدينا.
- `github.com/coreos/go-oidc/v3` و`golang.org/x/oauth2`: OIDC.
- TOTP هيتكتب بإيدينا (RFC 6238، حوالي 60 سطر) ومعاه اختبارات من الـ RFC، بدل مكتبة.

## مفتوح
- المراجعة الخارجية لمسارات المصادقة في M9 (ADR-0040)، والـ findings اللي فاضلة في قايمة الفحص.
