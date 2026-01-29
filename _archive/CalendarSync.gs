/**
 * ============================================================
 * CalendarSync.gs - 家庭内書類管理システム（カレンダー・タスク登録用）
 * ============================================================
 * お便りPDFをGeminiで解析し、予定やタスクを
 * Googleカレンダー・Google Tasksに自動登録する。
 * 
 * ※ 設定値はすべて Config.gs に記載
 * ============================================================
 */

// ============================================================
// MAIN FUNCTION
// ============================================================

/**
 * メイン処理：対象フォルダ内のファイルから予定・タスクを抽出して登録
 * ※ トリガーにはこの関数を設定してください
 */
function runCalendarSync() {
  // 夜間停止チェック（1:00〜6:00）
  if (isNightTime()) {
    Logger.log('夜間停止時間帯のため処理をスキップします。');
    return;
  }

  // 40_子供・教育配下のすべての対象フォルダを自動検索
  const targetFolders = findTargetFolders();
  Logger.log(`検出された対象フォルダ数: ${targetFolders.length}`);

  for (const folder of targetFolders) {
    try {
      Logger.log(`フォルダ処理開始: ${folder.getName()}`);
      processFolderObject(folder);
    } catch (error) {
      Logger.log(`フォルダ処理エラー（${folder.getName()}）: ${error.message}`);
    }
  }

  Logger.log('処理が完了しました。');
}

/**
 * 40_子供・教育配下の対象サブフォルダを検索
 * @returns {GoogleAppsScript.Drive.Folder[]} 対象フォルダの配列
 */
function findTargetFolders() {
  const targetFolders = [];
  const parentFolder = DriveApp.getFolderById(CHILDREN_EDU_FOLDER_ID);
  const childFolders = parentFolder.getFolders();

  // 各子供フォルダ（明日香、遥香など）を走査
  while (childFolders.hasNext()) {
    const childFolder = childFolders.next();
    
    // 各対象サブフォルダ名をチェック
    for (const subFolderName of TARGET_SUBFOLDER_NAMES) {
      const subFolders = childFolder.getFoldersByName(subFolderName);
      if (subFolders.hasNext()) {
        targetFolders.push(subFolders.next());
      }
    }
  }

  return targetFolders;
}

// ============================================================
// UTILITY FUNCTIONS
// ============================================================

/**
 * 夜間（1:00〜6:00）かどうかをチェック
 * @returns {boolean} 夜間の場合はtrue
 */
function isNightTime() {
  const hour = new Date().getHours();
  return hour >= 1 && hour < 6;
}

/**
 * フォルダ内のファイルを処理
 * @param {GoogleAppsScript.Drive.Folder} folder - フォルダオブジェクト
 */
function processFolderObject(folder) {
  const files = folder.getFiles();

  while (files.hasNext()) {
    const file = files.next();
    try {
      processFile(file, folder);
    } catch (error) {
      Logger.log(`ファイル処理エラー（${file.getName()}）: ${error.message}`);
    }
  }
}

/**
 * ファイルを処理：OCR → Gemini解析 → カレンダー/タスク登録 → processed移動
 * @param {GoogleAppsScript.Drive.File} file - 処理対象ファイル
 * @param {GoogleAppsScript.Drive.Folder} parentFolder - 親フォルダ
 */
function processFile(file, parentFolder) {
  const mimeType = file.getMimeType();
  const fileName = file.getName();

  // 対応ファイル形式をチェック
  if (!isSupportedFile(mimeType)) {
    Logger.log(`非対応のファイル形式をスキップ: ${fileName}`);
    return;
  }

  Logger.log(`処理開始: ${fileName}`);

  // OCRでテキスト抽出
  const ocrText = extractTextWithOCR(file);
  if (!ocrText || ocrText.length < 20) {
    Logger.log(`テキストが少なすぎるためスキップ: ${fileName}`);
    return;
  }

  // Geminiで予定・タスクを抽出
  const extractionResult = extractEventsAndTasks(ocrText, fileName);
  if (!extractionResult) {
    Logger.log(`Gemini解析に失敗: ${fileName}`);
    return;
  }

  Logger.log(`抽出結果: ${JSON.stringify(extractionResult)}`);

  const fileUrl = file.getUrl();

  // イベントをカレンダーに登録
  if (extractionResult.events && extractionResult.events.length > 0) {
    for (const event of extractionResult.events) {
      try {
        createCalendarEvent(event, fileUrl, fileName);
      } catch (error) {
        Logger.log(`イベント登録エラー: ${error.message}`);
      }
    }
  }

  // タスクをGoogle Tasksに登録
  if (extractionResult.tasks && extractionResult.tasks.length > 0) {
    for (const task of extractionResult.tasks) {
      try {
        createTask(task, fileUrl, fileName);
      } catch (error) {
        Logger.log(`タスク登録エラー: ${error.message}`);
      }
    }
  }

  // processedフォルダへ移動
  moveToProcessed(file, parentFolder);

  Logger.log(`処理完了: ${fileName}`);
}

/**
 * 対応ファイル形式かチェック
 * @param {string} mimeType - MIMEタイプ
 * @returns {boolean} 対応している場合はtrue
 */
function isSupportedFile(mimeType) {
  const supportedTypes = [
    'application/pdf',
    'image/jpeg',
    'image/png',
    'image/gif',
    'image/bmp'
  ];
  return supportedTypes.includes(mimeType);
}

/**
 * Drive APIのOCR機能でテキスト抽出
 * @param {GoogleAppsScript.Drive.File} file - 対象ファイル
 * @returns {string} 抽出されたテキスト（抽出不可の場合は空文字）
 */
function extractTextWithOCR(file) {
  try {
    const fileId = file.getId();

    // PDFまたは画像をGoogleドキュメントに変換（OCR）
    const resource = {
      title: `OCR_${file.getName()}_${Date.now()}`,
      mimeType: MimeType.GOOGLE_DOCS
    };

    const ocrFile = Drive.Files.copy(resource, fileId, {
      ocr: true,
      ocrLanguage: 'ja'
    });

    // Googleドキュメントからテキストを取得
    const doc = DocumentApp.openById(ocrFile.id);
    const text = doc.getBody().getText().trim();

    // 一時的なOCRドキュメントを削除
    DriveApp.getFileById(ocrFile.id).setTrashed(true);

    return text;
  } catch (error) {
    Logger.log(`OCRエラー: ${error.message}`);
    return '';
  }
}

/**
 * Gemini APIでイベント・タスクを抽出
 * @param {string} ocrText - OCRテキスト
 * @param {string} fileName - ファイル名
 * @returns {Object|null} 抽出結果JSON
 */
function extractEventsAndTasks(ocrText, fileName) {
  const today = Utilities.formatDate(new Date(), 'Asia/Tokyo', 'yyyy-MM-dd');

  const prompt = `
あなたは学校のお便りから予定とタスクを抽出するアシスタントです。
以下のOCRテキストを解析し、JSON形式で回答してください。

## 出力形式（必ずこのJSON形式で回答）
{
  "events": [
    {
      "title": "イベントタイトル",
      "date": "YYYY-MM-DD",
      "start_time": "HH:MM（不明な場合は null）",
      "end_time": "HH:MM（不明な場合は null）",
      "location": "場所（不明な場合は null）",
      "description": "詳細説明"
    }
  ],
  "tasks": [
    {
      "title": "タスクタイトル（例：○○の提出）",
      "due_date": "YYYY-MM-DD",
      "notes": "備考"
    }
  ]
}

## 判断基準
- **events**: 日時が確定している行事（運動会、授業参観、保護者会など）
- **tasks**: 期限がある提出物や準備事項（書類提出、持ち物準備など）

## 注意事項
- 過去の日付（${today}より前）のイベント・タスクは除外してください
- 年が明示されていない場合は、${today.substring(0, 4)}年と仮定してください
- 抽出できる情報がない場合は、eventsとtasksを空配列にしてください

## ファイル名
${fileName}

## OCRテキスト
${ocrText}
`;

  try {
    const url = `https://generativelanguage.googleapis.com/v1beta/models/gemini-1.5-flash:generateContent?key=${GEMINI_API_KEY}`;
    
    const payload = {
      contents: [{
        parts: [{ text: prompt }]
      }],
      generationConfig: {
        responseMimeType: 'application/json'
      }
    };

    const response = UrlFetchApp.fetch(url, {
      method: 'post',
      contentType: 'application/json',
      payload: JSON.stringify(payload),
      muteHttpExceptions: true
    });

    const result = JSON.parse(response.getContentText());
    
    if (result.error) {
      Logger.log(`Gemini APIエラー: ${result.error.message}`);
      return null;
    }

    const content = result.candidates[0].content.parts[0].text;
    return JSON.parse(content);
  } catch (error) {
    Logger.log(`Gemini解析エラー: ${error.message}`);
    return null;
  }
}

/**
 * Googleカレンダーにイベントを作成
 * @param {Object} event - イベント情報
 * @param {string} fileUrl - 元ファイルのURL
 * @param {string} fileName - 元ファイル名
 */
function createCalendarEvent(event, fileUrl, fileName) {
  const calendar = CalendarApp.getCalendarById(CALENDAR_ID);
  if (!calendar) {
    throw new Error(`カレンダーが見つかりません: ${CALENDAR_ID}`);
  }

  const description = `${event.description || ''}\n\n📎 元のお便り: ${fileUrl}`;

  // 時間の有無で終日イベントか時間指定イベントか判定
  if (event.start_time) {
    // 時間指定イベント
    const startDateTime = parseDateTime(event.date, event.start_time);
    const endDateTime = event.end_time 
      ? parseDateTime(event.date, event.end_time)
      : new Date(startDateTime.getTime() + 60 * 60 * 1000); // デフォルト1時間

    const options = {
      description: description,
      location: event.location || ''
    };

    calendar.createEvent(event.title, startDateTime, endDateTime, options);
    Logger.log(`イベント作成: ${event.title}（${event.date} ${event.start_time}）`);
  } else {
    // 終日イベント
    const eventDate = new Date(event.date);
    const options = {
      description: description,
      location: event.location || ''
    };

    calendar.createAllDayEvent(event.title, eventDate, options);
    Logger.log(`終日イベント作成: ${event.title}（${event.date}）`);
  }
}

/**
 * 日付と時間を解析してDateオブジェクトを作成
 * @param {string} dateStr - YYYY-MM-DD形式の日付
 * @param {string} timeStr - HH:MM形式の時間
 * @returns {Date} Dateオブジェクト
 */
function parseDateTime(dateStr, timeStr) {
  const [year, month, day] = dateStr.split('-').map(Number);
  const [hour, minute] = timeStr.split(':').map(Number);
  return new Date(year, month - 1, day, hour, minute);
}

/**
 * Google Tasksにタスクを作成
 * @param {Object} task - タスク情報
 * @param {string} fileUrl - 元ファイルのURL
 * @param {string} fileName - 元ファイル名
 */
function createTask(task, fileUrl, fileName) {
  const taskListId = '@default';  // デフォルトのタスクリスト

  const notes = `${task.notes || ''}\n\n📎 元のお便り: ${fileUrl}`;
  
  // RFC3339形式の期限日時
  const dueDate = new Date(task.due_date);
  dueDate.setHours(23, 59, 0);  // 期限日の23:59に設定

  const taskResource = {
    title: task.title,
    notes: notes.trim(),
    due: dueDate.toISOString()
  };

  Tasks.Tasks.insert(taskResource, taskListId);
  Logger.log(`タスク作成: ${task.title}（期限: ${task.due_date}）`);
}

/**
 * ファイルをprocessedフォルダへ移動
 * @param {GoogleAppsScript.Drive.File} file - 対象ファイル
 * @param {GoogleAppsScript.Drive.Folder} parentFolder - 親フォルダ
 */
function moveToProcessed(file, parentFolder) {
  // processedフォルダを取得または作成
  const processedFolder = getOrCreateFolder(parentFolder, 'processed');

  // 現在の親フォルダから削除し、processedフォルダに追加
  const parents = file.getParents();
  while (parents.hasNext()) {
    parents.next().removeFile(file);
  }
  processedFolder.addFile(file);

  Logger.log(`processedフォルダへ移動: ${file.getName()}`);
}

/**
 * サブフォルダを取得または作成
 * @param {GoogleAppsScript.Drive.Folder} parent - 親フォルダ
 * @param {string} name - フォルダ名
 * @returns {GoogleAppsScript.Drive.Folder} サブフォルダ
 */
function getOrCreateFolder(parent, name) {
  const folders = parent.getFoldersByName(name);
  if (folders.hasNext()) {
    return folders.next();
  }
  return parent.createFolder(name);
}
