/**
 * ============================================================
 * NotebookLMSync.gs - NotebookLM用シャドウドキュメント同期
 * ============================================================
 * Cloud Runで処理済みのファイルをNotebookLM用にOCR変換し、
 * 95_NotebookLM_Syncフォルダへカテゴリ別・年度別にコピーする。
 * 
 * ※ 設定値はすべて Config.gs に記載
 * ※ ファイル仕分けはCloud Runで実行（本スクリプトは同期のみ）
 * ============================================================
 */

// ============================================================
// CONFIGURATION
// ============================================================

/** シャドウコピー対象カテゴリ */
const SYNC_TARGET_CATEGORIES = [
  '20_プロジェクト・資産',
  '30_ライフ・行政',
  '40_子供・教育',
  '90_ライブラリ'
];

/** 処理済みマーカー（Driveプロパティに設定） */
const PROCESSED_MARKER = 'notebooklm_synced';

// ============================================================
// MAIN FUNCTION
// ============================================================

/**
 * メイン処理：対象カテゴリフォルダ内の未同期ファイルをNotebookLMへ同期
 * ※ トリガーにはこの関数を設定してください（例：1時間おき）
 */
function runNotebookLMSync() {
  // 夜間停止チェック（1:00〜6:00）
  if (isNightTime()) {
    Logger.log('夜間停止時間帯のため処理をスキップします。');
    return;
  }

  let totalProcessed = 0;

  // 各対象カテゴリを処理
  for (const category of SYNC_TARGET_CATEGORIES) {
    const folderId = CATEGORY_MAP[category];
    if (!folderId || folderId.startsWith('YOUR_')) {
      continue; // 未設定のフォルダはスキップ
    }

    try {
      const count = processCategory(category, folderId);
      totalProcessed += count;
      Logger.log(`${category}: ${count}件処理`);
    } catch (error) {
      Logger.log(`カテゴリ処理エラー（${category}）: ${error.message}`);
    }
  }

  Logger.log(`処理完了: 合計${totalProcessed}件`);
}

// ============================================================
// CATEGORY PROCESSING
// ============================================================

/**
 * カテゴリフォルダ内のファイルを処理
 * @param {string} category - カテゴリ名
 * @param {string} folderId - フォルダID
 * @returns {number} 処理件数
 */
function processCategory(category, folderId) {
  let count = 0;
  
  if (category === '40_子供・教育') {
    // 40_子供・教育は再帰的に処理（子供名/年度/サブカテゴリ構造）
    count = processFolderRecursive(category, folderId);
  } else {
    // その他は直下のファイルのみ
    count = processFilesInFolder(category, folderId);
  }
  
  return count;
}

/**
 * フォルダを再帰的に処理
 * @param {string} category - カテゴリ名
 * @param {string} folderId - フォルダID
 * @returns {number} 処理件数
 */
function processFolderRecursive(category, folderId) {
  let count = 0;
  const folder = DriveApp.getFolderById(folderId);
  
  // ファイルを処理
  count += processFilesInFolder(category, folderId);
  
  // サブフォルダを再帰処理
  const subFolders = folder.getFolders();
  while (subFolders.hasNext()) {
    const subFolder = subFolders.next();
    count += processFolderRecursive(category, subFolder.getId());
  }
  
  return count;
}

/**
 * フォルダ内のファイルを処理
 * @param {string} category - カテゴリ名
 * @param {string} folderId - フォルダID
 * @returns {number} 処理件数
 */
function processFilesInFolder(category, folderId) {
  let count = 0;
  const folder = DriveApp.getFolderById(folderId);
  const files = folder.getFiles();
  
  while (files.hasNext()) {
    const file = files.next();
    
    // 対応ファイル形式をチェック
    if (!isSupportedFile(file.getMimeType())) {
      continue;
    }
    
    // 処理済みチェック
    if (isAlreadySynced(file)) {
      continue;
    }
    
    try {
      // シャドウコピーを作成
      const result = createShadowDoc(file, category);
      if (result) {
        markAsSynced(file);
        count++;
      }
    } catch (error) {
      Logger.log(`ファイル処理エラー（${file.getName()}）: ${error.message}`);
    }
  }
  
  return count;
}

// ============================================================
// SHADOW DOCUMENT CREATION
// ============================================================

/**
 * NotebookLM用シャドウドキュメントを作成
 * PDF/画像をOCRでGoogleドキュメントに変換して保存
 * @param {GoogleAppsScript.Drive.File} file - 元ファイル
 * @param {string} category - カテゴリ名
 * @returns {boolean} 成功したかどうか
 */
function createShadowDoc(file, category) {
  const fileId = file.getId();
  const fileName = file.getName();
  
  // ファイル名から日付を抽出（YYYYMMDD_タイトル.ext形式を想定）
  const dateMatch = fileName.match(/^(\d{8})_/);
  const date = dateMatch ? dateMatch[1] : Utilities.formatDate(new Date(), 'Asia/Tokyo', 'yyyyMMdd');
  const title = fileName.replace(/^\d{8}_/, '').replace(/\.[^.]+$/, '');
  
  const docName = `【${category}】${date}_${title}`;
  
  // 保存先フォルダを決定
  const syncFolder = DriveApp.getFolderById(FOLDER_IDS.NOTEBOOKLM_SYNC);
  let destFolder;
  
  if (category === '40_子供・教育') {
    // 40_子供・教育は年度別に分類
    const categoryFolder = getOrCreateFolder(syncFolder, category);
    const fiscalYear = getFiscalYear(date);
    destFolder = getOrCreateFolder(categoryFolder, `${fiscalYear}年度`);
  } else {
    // その他のカテゴリはカテゴリフォルダ直下
    destFolder = getOrCreateFolder(syncFolder, category);
  }
  
  // Drive APIでPDF/画像をGoogleドキュメントに変換（OCR）
  const resource = {
    title: docName,
    parents: [{ id: destFolder.getId() }]
  };

  const shadowDoc = Drive.Files.copy(resource, fileId, {
    convert: true,
    ocr: true,
    ocrLanguage: 'ja'
  });

  Logger.log(`シャドウドキュメント作成: ${docName}`);
  return true;
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
 * 日付文字列（YYYYMMDD）から日本の学校年度を取得
 * ※ 4月始まり、1〜3月は前年度扱い
 * @param {string} dateString - YYYYMMDD形式の日付
 * @returns {number} 年度（例: 2025）
 */
function getFiscalYear(dateString) {
  const year = parseInt(dateString.substring(0, 4), 10);
  const month = parseInt(dateString.substring(4, 6), 10);
  
  // 1〜3月は前年度扱い
  if (month >= 1 && month <= 3) {
    return year - 1;
  }
  return year;
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
 * ファイルが既に同期済みかチェック
 * @param {GoogleAppsScript.Drive.File} file - ファイル
 * @returns {boolean} 同期済みの場合はtrue
 */
function isAlreadySynced(file) {
  try {
    const properties = Drive.Properties.list(file.getId());
    for (const prop of properties.items || []) {
      if (prop.key === PROCESSED_MARKER && prop.value === 'true') {
        return true;
      }
    }
    return false;
  } catch (error) {
    return false;
  }
}

/**
 * ファイルを同期済みとしてマーク
 * @param {GoogleAppsScript.Drive.File} file - ファイル
 */
function markAsSynced(file) {
  try {
    Drive.Properties.insert(
      { key: PROCESSED_MARKER, value: 'true', visibility: 'PRIVATE' },
      file.getId()
    );
  } catch (error) {
    Logger.log(`マーキングエラー: ${error.message}`);
  }
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
