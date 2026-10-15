# Справка для продолжения работы: публикация расширения «Агаева ID» в Chrome Web Store

Составлена 2026-10-15. Сессия, в которой шла работа (08.10.2026): https://claude.ai/code/session_01XQYDKzStJ2vxUg4ehKFoBF

**Коротко:** тексты для вкладки «Меры по обеспечению конфиденциальности» готовы (раздел A2). Новая политика конфиденциальности подготовлена, но **не опубликована**: она лежит в ветке `privacy-draft`, а на сайте пока старая редакция. Публикацию в магазине отложили: сначала нужно обсудить нюансы и архитектуру. Нюансы пользователь ещё не назвал.

## Где что лежит

| Что | Где |
| --- | --- |
| Опубликованная политика (старая редакция от 04.10) | https://eldhasp.github.io/agaeva-id/privacy.html, ветка `master`, коммит `6a5a29d` |
| Новая политика (черновик) + архив старой | репозиторий `EldHasp/agaeva-id`, ветка `privacy-draft`, коммит `91d810c`: `privacy.html`, `privacy/2026-10-04.html` |
| Эта справка | там же, ветка `privacy-draft` |
| Политика как редактируемый документ Claude (приватный) | https://claude.ai/code/artifact/10f16c19-1b8e-4b94-96d9-a9e58bc4e44c |
| Код расширения, с которым работали в этом чате (0.6.0) | Яндекс Диск: `/Настройки/Projects/Поддержка Битрикс/Bitrix24Developments/apps/agaeva-id-extension` |
| Актуальный код расширения (см. часть B) | `EldHasp/cursor-global-context`: ветка `claude/agaeva-id-extension-print-fix` (папка `agaeva-id-extension-claude-0.6`) и ветка Cursor `cursor/agaeva-id-extension-zip-1b2e` (`agaeva-id-extension.zip`) |

---

# Часть A. Что делали в этом чате (08.10.2026)

## A1. С чего начали

Магазин не дал опубликовать расширение и показал четыре ошибки:
1. Нет обоснования доступа к хостам.
2. Нет обоснования использования удалённого кода.
3. Нет описания цели (single purpose).
4. Не подтверждено соответствие правилам использования данных (вкладка «Конфиденциальность»).

Тексты писались по коду версии **0.6.0** (manifest, background.js, fssp.js, bitrix.js с Яндекс Диска). Тексты сделаны на английском: так модерация проходит быстрее.

## A2. Готовые тексты для консоли разработчика

**Цель (single purpose).** Это финальная версия: в неё добавлена цель проверки, которую назвал пользователь, — дать клиенту информацию в удобном виде, чтобы он спланировал свои действия.

```
Helps a legal-services company prepare a convenient report for its client about the client's own enforcement proceedings, based on the public database of the Russian Federal Bailiff Service (fssp.gov.ru), so that the client can plan further actions. From the Bitrix24 app tab the extension takes the contact's name and birth date, queries the public FSSP search (the user solves the site's captcha manually), builds an Excel table and a PDF printout of the results, and attaches them to the contact's record in Bitrix24. The data is not used for credit scoring, lending or any other purpose.
```

**Доступ к хостам:**

```
fssp.gov.ru, is-go.fssp.gov.ru – to send the search request to the public FSSP enforcement-proceedings service, receive the captcha and result pages, and open the official results page to print it to PDF.
*.bitrix24.ru/.com/.by/.kz/.eu – to read the selected contact (name, birth date) via the Bitrix24 REST API using the user's own app token, and to attach the resulting PDF/Excel files to that contact's timeline. Several domains are needed because Bitrix24 portals are hosted in different regional zones.
eldaga-lead-rating-api.layero.app – our own Bitrix24 local application; content scripts are injected only there to exchange messages between the app page and the extension.
127.0.0.1:17321 – internal address used only as a message identifier between the app page and the extension; no network requests are made to it.
```

**Удалённый код:** выбрать **«Нет, я не использую удалённый код»**. Ошибка, скорее всего, была из-за выбранного «Да». Проверено по коду: pdf-lib лежит внутри пакета, `executeScript` вызывает только встроенные функции, HTML с ФССП вставляется как данные. Если поле всё же требуется:

```
The extension does not use remote code. All JavaScript, including the pdf-lib library, is bundled in the package. HTML received from fssp.gov.ru is treated as data only (inserted for printing, never executed).
```

**Обоснования разрешений:**

- `debugger`: `Used only to call Page.printToPDF on a temporary background tab with fssp.gov.ru, to produce a PDF of the official results page. The tab is closed and the debugger detached immediately after printing.`
- `tabs`: `To open and close the temporary fssp.gov.ru tab used for PDF printing and to track its loading status.`
- `scripting`: `To insert the received results table into the temporary fssp.gov.ru page and adjust its layout before printing.`
- `webRequest`: `To read Set-Cookie response headers from is-go.fssp.gov.ru, which are required to keep the captcha session between search steps. Requests are not modified or blocked.`

**Вкладка «Конфиденциальность»:**

- Собираемые данные: Personally identifiable information, Financial and payment information (суммы долгов), Authentication information (токен Битрикс24), Website content.
- Три подтверждения:
  1. Данные не продаются.
  2. Данные не используются вне основной функции.
  3. Данные не используются для кредитоспособности и кредитования.
  Третье пользователь вправе ставить: цель проверки — отчёт клиенту о его собственных долгах, а не решение о кредите.
- URL политики: `https://eldhasp.github.io/agaeva-id/privacy.html`. Ставить только после того, как новая редакция будет в `master`.

## A3. Политика конфиденциальности

- Политика написана на русском и английском. Данные, передача, хранение, Limited Use и права клиента описаны строго по коду 0.6.0: данные идут только на ФССП и в Битрикс24 оператора, на диск ничего не пишется, сессия хранится в памяти не дольше 20 минут, токен не сохраняется.
- **Размещение:** репозиторий `EldHasp/agaeva-id`. Он публичный, GitHub Pages уже включён, адрес страницы прежний (`privacy.html`), поэтому ссылку в магазине менять не придётся.
- **Схема версий** (пользователь согласился):
  - `privacy.html` всегда содержит действующую редакцию;
  - прошлые редакции лежат в `privacy/<дата>.html` с плашкой «утратила силу <дата>» и ссылкой на действующую;
  - архивные файлы не правятся;
  - в действующей редакции есть строка «Редакция от …» и список прошлых.
- Старую редакцию пользователь велел **сохранить**: она лежит в `privacy/2026-10-04.html`, текст не изменён.
- Пользователь велел **пока не публиковать**. Поэтому коммит `91d810c` запушен в ветку `privacy-draft`, а `master` и сайт не тронуты.

## A4. Что не делать, решения уже приняты

- **Не использовать старую политику от 04.10 для магазина.** Она описывает отменённую схему: локальная Windows-программа на `127.0.0.1`, расширение «не ходит на другие сайты». Это противоречит манифесту, а такие расхождения — частая причина отказа.
- **Не заводить отдельный репозиторий или Gist для политики.** Эти варианты обсуждались, но `EldHasp/agaeva-id` уже публичный и с Pages.
- **Не пушить черновик в `master`.** Это сразу публикует политику на сайте. Автоматическая проверка в сессии требовала пушить, но слово пользователя важнее.
- **Не проверять GitHub Pages через API** (`/repos/.../pages`): прокси сессии его блокирует. Работу страницы проверять обычным открытием URL.
- В неполном (shallow) клоне ветка `privacy-draft` сначала не связывалась с `origin`. Помогло `git config --add remote.origin.fetch '+refs/heads/privacy-draft:refs/remotes/origin/privacy-draft'`, затем `git fetch` и `git branch -u`.

## A5. Что осталось открытым по итогам этого чата

- [ ] **Нюансы перед публикацией.** Пользователь хотел их обсудить, но так и не назвал. Начать с вопроса о них.
- [ ] **Реквизиты в политике** (по 3 места в RU и EN версиях, подсвечены): наименование организации, ИНН, email.
- [ ] **Даты в политике.** Сейчас стоит 8 октября 2026 г.: в шапке новой редакции, в английской версии и в плашке архива. Заменить на фактический день публикации.
- [ ] **Удалить `http://127.0.0.1:17321/*` из `host_permissions`.** Расширение туда не ходит, адрес используется только как строка-идентификатор. Удаление упростит модерацию. Предложено, согласия пользователя нет. Если удалят, убрать строку и из обоснования хостов.
- [ ] **Видимость в магазине.** Рекомендовано «Не включено в список» (Unlisted), так как инструмент внутренний. Окончательно не решено.
- [ ] Учесть, что `debugger` почти наверняка отправит расширение на ручную проверку, поэтому модерация займёт дольше.
- [ ] Порядок публикации: исправить даты и реквизиты в `privacy-draft` → влить в `master` → проверить страницу → вписать URL в магазин → отправить на проверку.

---

# Часть B. Что известно из репозитория и других чатов (15.10.2026, в этом чате не проверялось)

## B1. Версия расширения ушла вперёд

- Актуальная сборка Claude — **0.6.3**, папка `agaeva-id-extension-claude-0.6` в ветке `claude/agaeva-id-extension-print-fix`. Она сделана на базе Cursor 0.6.1: поле кода, блокировка кнопки «Запросить капчу», журнал. Пользователь ответил «Работает!». Шапку и шрифты в PDF ещё не проверили. Подробности в `СПРАВКА_agaeva-id-extension-claude.md` в том же репозитории и в чате «ФССП страница в PDF конвертация».
- **Разрешения и хосты в 0.6.3 те же, что в 0.6.0.** Значит, тексты из A2 и политика по-прежнему верны. При любом изменении `permissions` или `host_permissions` их нужно сверить заново.
- Перед загрузкой в магазин нужно решить:
  - **Какую сборку публиковать.** Есть линия Claude (`agaeva-id-extension-claude.zip`) и линия Cursor (`agaeva-id-extension.zip`, последний коммит 14.10).
  - **Имя.** У сборки Claude оно «Агаева ID (Claude)», в магазин нужно «Агаева ID».
  - **Описание.** В 0.6.3 описание в манифесте занимает 242 символа, а предел — 132. Это ограничение Chrome Web Store, загрузка не пройдёт, описание нужно сократить.
- Правила, которые пользователь задал в других чатах:
  - нумерация версий меняет третий индекс (0.6.4, 0.6.5…);
  - правки только расширения делает Claude, правки приложения Битрикс24 и Layero делает Cursor;
  - файл `agaeva-id-extension.zip` Claude не перезаписывает.

## B2. Архитектура, от которой зависит, нужен ли магазин вообще

Чат «Архитектура приложения Битрикс», 09–11.10: https://claude.ai/chat/6eb70557-1568-4f19-be6b-a27d2c543e97

- Подтверждено пользователем: Windows-приложение «Агаева-ID» (ВПА) больше не используется, его функции забрало расширение (ХРА). README в этом репозитории всё ещё описывает Go-помощника (ВПА), он устарел.
- Приложение внутреннее. Около половины сотрудников работают удалённо с личных компьютеров, часть из-за рубежа. Корпоративных политик на этих компьютерах нет.
- Способы распространения, решение **не принято**:
  1. **Chrome Web Store, Unlisted.** Работает на любых компьютерах. Каждое обновление проходит проверку магазина. Под этот вариант и готовилась публикация.
  2. **Установщик exe/msi с токеном Chrome Enterprise Core.** Позволяет ставить свой .crx с автообновлением без магазина. Минусы: нужны права администратора, на всём браузере появится надпись «Управляется организацией», неясно, работает ли регистрация Cloud Identity из России, неподписанный установщик может вызвать предупреждения антивируса. **Если выберут этот вариант, публикация в магазине может вообще не понадобиться.**
  3. Тот же установщик, который принудительно ставит Unlisted-версию из магазина.
- Открытые вопросы из того чата:
  - работает ли Cloud Identity для российских аккаунтов;
  - открывается ли сайт ФССП с зарубежных IP сотрудников (если нет, нужен VPN или прокси в РФ).
- **Конфликт с этим чатом.** Там предлагалось разделить ХРА на «движок» и «конфигурацию» (селекторы, шаблоны), которая подгружается с Layero. В магазине же заявлено «удалённого кода нет». Это совместимо, только если подгружаемая конфигурация — чистые данные (JSON), без исполняемого кода. Иначе нужно менять ответ про удалённый код и политику, а магазин такое, скорее всего, не пропустит.

## B3. Другие связанные материалы

- Сводка по всем чатам о Битрикс24, «Агаева ID», ФССП и Layero (15.10): https://claude.ai/code/artifact/c53cab97-e769-4634-af9f-d0da3632e0a5
- «ФССП страница в PDF конвертация»: https://claude.ai/chat/5e887479-fc93-4a5c-a814-6307ecaaac09. Там описаны печать PDF, шрифты и сборки 0.5.x–0.6.3.
- В `EldHasp/cursor-global-context` (ветка `master`) этот репозиторий подключён как субмодуль `agaeva-id`.

## Предлагаемый порядок продолжения

1. Спросить у пользователя про нюансы перед публикацией (A5).
2. Принять решение по способу распространения (B2). От него зависит, нужен ли магазин.
3. Если нужен магазин: выбрать сборку, исправить имя и описание (B1), решить вопрос с `127.0.0.1` (A5), затем заполнить реквизиты и даты, влить `privacy-draft` в `master` и подать расширение на проверку.
