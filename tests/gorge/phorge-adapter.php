<?php

// Test driver only. The message, adapter, serializer and transport below are
// original classes from the pinned, clean Phorge checkout, not local stand-ins.
error_reporting(E_ALL);
ini_set('display_errors', 'stderr');
date_default_timezone_set('UTC');

try {
  if (!extension_loaded('curl') || !extension_loaded('mbstring')) {
    throw new RuntimeException('PHP curl and mbstring extensions are required.');
  }
  $input = json_decode(stream_get_contents(STDIN), true, 512, JSON_THROW_ON_ERROR);
  $root = getenv('OWLMAIL_PHORGE_SOURCE');
  if (!$root || $root[0] !== '/') {
    throw new RuntimeException('An absolute pinned Phorge source is required.');
  }
  if (!preg_match('@\Ahttp://127\.0\.0\.1:[1-9][0-9]{0,4}\z@', $input['uri'])) {
    throw new RuntimeException('The test may submit only to a loopback Gorge.');
  }
  require_once $root.'/support/runtime/bootstrap.php';
  phutil_load_library($root.'/src');
  // Deliberately do not initialize PhabricatorEnv: that loads database/local
  // configuration. These injected-endpoint message/adapter classes need only
  // the original bundled runtime and class library.

  $data = $input['message'];
  $message = new PhabricatorMailEmailMessage();
  $message->setFromAddress(new PhutilEmailAddress($data['from']['address']));
  foreach (array('to' => 'setToAddresses', 'cc' => 'setCCAddresses') as $field => $method) {
    $addresses = array();
    foreach ($data[$field] as $address) {
      $addresses[] = new PhutilEmailAddress($address['address']);
    }
    $message->$method($addresses);
  }
  $message->setSubject($data['subject']);
  $message->setTextBody($data['textBody']);
  $message->setHTMLBody($data['htmlBody']);
  $headers = array();
  foreach ($data['headers'] as $header) {
    $headers[] = id(new PhabricatorMailHeader())
      ->setName($header['name'])->setValue($header['value']);
  }
  $message->setHeaders($headers);
  $attachments = array();
  foreach ($data['attachments'] as $attachment) {
    $bytes = base64_decode($attachment['data'], true);
    if ($bytes === false) {
      throw new RuntimeException('Invalid synthetic attachment base64.');
    }
    $attachments[] = new PhabricatorMailAttachment(
      $bytes, $attachment['filename'], $attachment['mimeType']);
  }
  $message->setAttachments($attachments);

  $adapter = new PhabricatorMailGorgeAdapter();
  $adapter->setOptions(array_merge($adapter->newDefaultOptions(), array(
    'uri' => $input['uri'],
    'token' => $input['token'],
    'timeout' => 10,
    'supports-message-id' => true,
  )));
  $origins = array();
  foreach (array(
    'PhabricatorMailGorgeAdapter',
    'PhabricatorGorgeMailerClient',
    'PhabricatorGorgeServiceClient',
    'PhabricatorMailEmailMessage',
    'HTTPSFuture',
  ) as $class) {
    $reflection = new ReflectionClass($class);
    $file = realpath($reflection->getFileName());
    if (strpos($file, realpath($root).DIRECTORY_SEPARATOR) !== 0) {
      throw new RuntimeException('A class came from outside pinned Phorge.');
    }
    $origins[$class] = substr($file, strlen(realpath($root)) + 1);
  }
  $classification = 'accepted';
  try {
    $adapter->sendMessage($message);
  } catch (PhabricatorMetaMTAPermanentFailureException $ex) {
    $classification = 'permanent';
  }
  echo json_encode(array(
    'classification' => $classification,
    'adapter' => get_class($adapter),
    'phpVersion' => PHP_VERSION,
    'classes' => $origins,
  ), JSON_THROW_ON_ERROR)."\n";
} catch (Throwable $ex) {
  fwrite(STDERR, get_class($ex).': '.$ex->getMessage()."\n");
  exit(1);
}
