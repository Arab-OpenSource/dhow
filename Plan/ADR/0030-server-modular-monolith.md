# ADR-0030: هيكل السيرفر: Modular Monolith بتكييف Go

- **الحالة:** مقبول
- **التاريخ:** 2026-10-27

## السياق
أول نسخة من السيرفر كانت packages مسطّحة (`api` و`httpx` و`db` و`server`). كل ميزة جديدة كانت هتزوّد الملفات في نفس المجلدات. وطبقات `Stacks/Go.md` (handlers وservices وrepository) بتجمّع كل الميزات في مجلد واحد لكل طبقة. Sabry عنده نمط Modular Monolith في `Stacks/Bun-Hono.md` وعايزه هو الأساس هنا، متكيّف مع قواعد Go.

## القرار
- **الهيكل:** `apps/server/internal` فيه `App` و`Config` و`Core/<X>` و`Database/<X>` و`Modules/<Name>`. المجلدات وملفات Go بـ PascalCase ونقط للأدوار (`System.Service.go`). أسماء الـ packages تفضل lowercase لأن Go بتطلب كده. `Core/Errors` هو `package apperrors` عشان ما يتخبطش مع `errors`.
- **كل موديول لوحده:** Routes وController وService وModels وQueries وGen وTest. الـ Test فيه Unit وIntegration وE2E وEdge، كلها black-box. الاختبار بتاع Postgres بيتخطى لو `DHOW_TEST_DATABASE_URL` مش موجود.
- **اتجاه الاستيراد:** `App` بس بيستورد `Modules`. الموديول ما يستوردش موديول تاني. `Core` و`Config` و`Database` ما يستوردوش `Modules` ولا `App`. والـ hook بيمنع أي استيراد مخالف.
- **التواصل بين الموديولات:** `Core/EventBus`. ده bus بسيط جوه العملية، الـ handlers بتشتغل بالترتيب في نفس الـ goroutine. أخطاؤها بتتسجّل وبترجع للمرسِل، ومفيش goroutines بتتسرّب.
- **Codegen لكل موديول:** ملف `oapi-codegen.yaml` في كل موديول بـ `include-tags`. فكل موديول بيملك الـ routes والـ strict interface بتوعه. وsqlc بيقرا مجلد `Queries` في كل الموديولات ويطلّع في `Database/Sqlc`. `make gen` بيشغّل الكل.
- **الـ envelope:** `{Success, Data?, Message?, Meta?, Error?}` والحقول الفاضية بتتشال (`omitempty`) مش بتبقى null. الأخطاء بـ `Code` UPPER_SNAKE (`NOT_FOUND`، `VALIDATION_FAILED`...). رسالة الخطأ بتتترجم في السيرفر من `Accept-Language` والعربي هو الافتراضي والإنجليزي بديل. رسالة النجاح مفتاح i18n زي `system.meta.loaded` والواجهة هي اللي بتترجمه. مفيش نص خطأ داخلي بيوصل للعميل.
- **قاعدة البيانات:** كل جدول جديد فيه `"Id" uuid DEFAULT uuidv7()` و`"CreatedAt"` و`"UpdatedAt"` و`"IsDeleted"`. الحذف soft بس. الوقت `timestamptz` بـ UTC، والنص `TEXT`، والترجمات في جدول `<Entity>Translations` مش أعمدة `NameAr` و`NameEn`.
- **PostgreSQL 18:** عشان `uuidv7()` مدمجة. صورة الـ CI بقت `postgres:18`.
- **استثناء `"Instance"`:** ده singleton (`"Id"` رقم 1 وبدون `IsDeleted`) وهو الجدول الوحيد المستثنى من الأعمدة الأساسية. والـ hook عنده قايمة استثناء بالاسم ده بس.
- **المسارات:** أجزاء الـ API بـ kebab-case. `openapi.yaml` بيتعدّل الأول وبعدين `make gen` وبعدين الكود.
- **داخل Go بيفضل camelCase:** في Go الحرف الكبير معناه exported. فاللي مش exported (`readinessTimeout`، `statusRecorder`) بيفضل camelCase، واللي exported بـ PascalCase. الـ JSON والجداول والأعمدة بـ PascalCase زي ما هي.
- **الـ hooks:** مكان ملفات Go وتسميتها واتجاه الاستيراد وقواعد الـ migrations وممنوع `DELETE FROM` في `Queries` ومسارات kebab-case. ولكل قاعدة اختبار في `.claude/hooks/tests/`.
- **المصادقة بعدين:** `argon2id` (من `golang.org/x/crypto/argon2`) و`go-jose` هيجوا مع موديول المصادقة. ما اتضافوش دلوقتي لأن مفيش auth لسه.
- **الاعتماديات:** `go-playground/validator` للتحقق من الـ config (بيفشل بدل ما يكمّل، وما بيطبعش قيم الأسرار)، و`testify` للاختبارات.

## البدائل اللي اتدرست
- **طبقات `Stacks/Go.md`:** مرفوضة. كل ميزة بتزوّد في مجلد الـ handlers والـ services والـ repository، فالمجلد الواحد بيكبر مع كل ميزة. وكمان فصل `ee/` أصعب، لأن ميزة تجارية هتتوزع على ثلاث طبقات. مع الموديولات، ميزة `ee/` موديول كامل بيركّبه `App`.
- **Microservices من الأول:** تعقيد تشغيل مالوش لازمة، والمنتج بيتنصّب عند العميل كـ binary واحد.
- **Codegen واحد للـ API كله:** ملف `gen` واحد بيجبر كل الموديولات تعدّل في نفس المكان، وبيكسر ملكية الموديول لـ routes بتاعته.

## العواقب
- إضافة ميزة جديدة = مجلد موديول جديد وسطر واحد في `App`.
- الـ schemas المشتركة (`Envelope` و`ErrorDetail`) بتتولّد نسخة في كل موديول لأن كل موديول عنده package `gen` بتاعه. ده تكرار بسيط في كود متولّد.
- الأسماء بـ PascalCase في المجلدات والملفات غير معتادة في Go، والـ hook هو اللي بيفرضها.
- بما إن الموديولات ما بتستوردش بعض، التواصل بالأحداث معناه إن الـ flow أصعب شوية في التتبع. وبنقبل ده مقابل الفصل.
- لازم PostgreSQL 18 أو أحدث في التطوير والتشغيل.
