# VKontakte &nbsp;·&nbsp; [🇷🇺 RU](VK_RU.md)

## 1. Obtain a call ID
To make sure its impossible to trace you back to your VK account, just [search  `"vk.com/call/join"` on Google](https://www.google.com/search?q=%22vk.com%2Fcall%2Fjoin%22):
![Example](https://files.catbox.moe/ic6sf9.png)

The part that comes after `https://vk.com/call/join/` is the call ID. 

## 2. Deal with the captcha
We're trying our best to make sure the automatic captcha solver works fine, but in case its currently broken, do this:

### 2.1. Install the userscript
Once all automatic captcha solving attempts fail, you will see something this:
```
2026-04-22 16:09:19.748 [INFO] manual captcha solve required userscript=http://localhost:1984/vk_manual_captcha.user.js guide=http://localhost:1984/ url=https://vk.com/call/join/... timeout=10m0s
```

First, you need to install [TamperMonkey](https://addons.mozilla.org/en-US/firefox/addon/tampermonkey/). The link to the Firefox extension page was provided intentionally, as Firefox allows to install and make use of extensions even on mobile platforms, e.g. Android. Make sure to give it permission to run in Incognito.
![Tampermonkey](https://files.catbox.moe/y5vlyg.png)

Afterward, copy the `userscript` URL from the log line and open it in your browser. You will see something like this:
![Installation](https://files.catbox.moe/pq98f9.png)

Click the install button.

### 2.2. Finish the captcha
Now proceed to the `url` URL and finish the captcha manually. The userscript will automatically click the Join button for you, and will redirect you off of VK once it successfully captures all the necessary tokens. 

If you are logged in, use Incognito. If you are on Android and have the VK app intalled, use Incognito as well, as otherwise it would redirect you to it. Enable desktop mode.

## 3. PROFIT!
That is all you need to do.
