/**
 * Cloud Run Document Processor Trigger
 * 
 * 指定されたフォルダ内の新しいファイルを検出し、Cloud RunにPub/Sub経由で通知します。
 * 
 * セットアップ手順:
 * 1. このスクリプトをGASプロジェクトにコピー
 * 2. プロジェクト設定 > スクリプトプロパティ に以下を設定:
 *    - PROJECT_ID: GCPプロジェクトID (bright-lattice-328909)
 *    - TOPIC_NAME: drive-events
 *    - SOURCE_FOLDER_ID: 監視対象のフォルダID (1T_XJURJbSsSiarr2Y-ofH0lCpSn9Dmak)
 * 3. サービス設定 > Pub/Sub API を有効化
 * 4. トリガー設定: checkNewFiles関数を「時間主導型」で1分おきに実行
 */

function checkNewFiles() {
  const props = PropertiesService.getScriptProperties();
  const folderId = props.getProperty('SOURCE_FOLDER_ID') || '1T_XJURJbSsSiarr2Y-ofH0lCpSn9Dmak';
  
  // 処理済みファイルの時間を記録・取得
  const lastCheckTimeStr = props.getProperty('LAST_CHECK_TIME');
  // 初回実行時は現在時刻-1分を開始点とする
  let lastCheckTime = lastCheckTimeStr ? new Date(parseInt(lastCheckTimeStr)) : new Date(Date.now() - 60 * 1000);
  const now = new Date();
  
  // 検索クエリ: 指定時間以降に作成され、かつゴミ箱に入っていないファイル
  // 検索クエリ: シンプルにゴミ箱に入っていないファイルのみを検索
  // 日付のフィルタリングは、クエリエラーを避けるためにスクリプト内で行います
  const folder = DriveApp.getFolderById(folderId);
  const query = 'trashed = false';
  console.log(`Search Query: ${query}`);
  const files = folder.searchFiles(query);
  
  while (files.hasNext()) {
    const file = files.next();
    
    // 日付チェック (スクリプト内でフィルタリング)
    // ファイルの作成日時が、前回のチェック日時より新しい場合のみ処理
    if (file.getDateCreated().getTime() <= lastCheckTime.getTime()) {
      continue;
    }
    
    // PDFまたは画像のみ対象
    const mimeType = file.getMimeType();
    if (mimeType === 'application/pdf' || mimeType.startsWith('image/')) {
      console.log(`Processing file: ${file.getName()} (${file.getId()})`);
      publishMessage(file.getId());
    }
  }
  
  // チェック時刻を更新
  props.setProperty('LAST_CHECK_TIME', now.getTime().toString());
}

/**
 * Pub/Subにメッセージを送信
 */
function publishMessage(fileId) {
  const props = PropertiesService.getScriptProperties();
  const projectId = props.getProperty('PROJECT_ID') || 'bright-lattice-328909';
  const topicName = props.getProperty('TOPIC_NAME') || 'drive-events';
  
  const url = `https://pubsub.googleapis.com/v1/projects/${projectId}/topics/${topicName}:publish`;
  
  // ペイロード作成
  // Cloud Run側は {'message': {'data': base64encoded_json}} を期待しているが、
  // Pub/Sub APIに直接投げる場合は API形式に合わせる
  
  const dataPayload = JSON.stringify({ file_id: fileId });
  const dataBase64 = Utilities.base64Encode(dataPayload);
  
  const payload = {
    messages: [
      {
        data: dataBase64
      }
    ]
  };
  
  const options = {
    method: 'post',
    contentType: 'application/json',
    headers: {
      Authorization: 'Bearer ' + ScriptApp.getOAuthToken()
    },
    payload: JSON.stringify(payload)
  };
  
  try {
    const response = UrlFetchApp.fetch(url, options);
    console.log(`Published ${fileId}: ${response.getResponseCode()}`);
  } catch (e) {
    console.error(`Failed to publish ${fileId}: ${e}`);
  }
}

/**
 * 前回のチェック時間をリセットするユーティリティ関数
 * これを実行すると、次回実行時にInbox内の全てのファイルが処理対象になります。
 */
function resetCheckTime() {
  PropertiesService.getScriptProperties().deleteProperty('LAST_CHECK_TIME');
  console.log('LAST_CHECK_TIME をリセットしました。次回実行時にすべてのファイルを処理対象にします。');
}

/**
 * 過去のファイルも含めて強制的にすべて処理対象にする設定関数
 * これを実行後に checkNewFiles を動かすと、Inbox内の古いファイルも処理されます。
 */
function setCheckTimePast() {
  PropertiesService.getScriptProperties().setProperty('LAST_CHECK_TIME', new Date('2020-01-01').getTime().toString());
  console.log('LAST_CHECK_TIMEを2020年に設定しました。');
}

/**
 * デバッグ用：対象フォルダの中身と、判定ロジックの結果をログに出力します。
 * ファイルがなぜスキップされているか特定するのに使用してください。
 */
function debugFolderContent() {
  const props = PropertiesService.getScriptProperties();
  const folderId = props.getProperty('SOURCE_FOLDER_ID') || '1T_XJURJbSsSiarr2Y-ofH0lCpSn9Dmak'; // Default to 00_Inbox
  const lastCheckTimeStr = props.getProperty('LAST_CHECK_TIME');
  // checkNewFilesと同じロジックで基準日時を決定
  const lastCheckTime = lastCheckTimeStr ? new Date(parseInt(lastCheckTimeStr)) : new Date(Date.now() - 60 * 1000);

  console.log(`--- DEBUG START ---`);
  console.log(`Target Folder ID: ${folderId}`);
  try {
    const folder = DriveApp.getFolderById(folderId);
    console.log(`Folder Name: ${folder.getName()}`);
    
    console.log(`Last Check Time: ${lastCheckTime.toString()} (Timestamp: ${lastCheckTime.getTime()})`);
    
    const files = folder.searchFiles('trashed = false');
    let count = 0;
    
    while (files.hasNext()) {
      const file = files.next();
      count++;
      const created = file.getDateCreated();
      const mimeType = file.getMimeType();
      
      console.log(`[File ${count}] ${file.getName()}`);
      console.log(`  Created: ${created.toString()} (${created.getTime()})`);
      console.log(`  MimeType: ${mimeType}`);
      
      // 日付判定チェック
      if (created.getTime() <= lastCheckTime.getTime()) {
        console.log(`  -> 判定: SKIP (日付が古い)`);
      } else {
        // 形式判定チェック
        if (mimeType === 'application/pdf' || mimeType.startsWith('image/')) {
          console.log(`  -> 判定: OK (処理対象になります)`);
        } else {
          console.log(`  -> 判定: SKIP (対応していない形式)`);
        }
      }
    }
    
    if (count === 0) {
      console.log('フォルダ内にファイルが見つかりませんでした。');
    }
  } catch (e) {
    console.error(`フォルダアクセスエラー: ${e.message}`);
  }
  console.log(`--- DEBUG END ---`);
}
