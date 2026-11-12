var trymosml = function(){
    // Exported functions
    extern = {};
    extern.makeController = makeController;
    extern.connectWebsocket = connectWebsocket;
    extern.makeEditor = makeEditor;
    extern.sendEditorContent = sendEditorContent;
    extern.resetConsole = resetConsole;

    extern.newFile = newFile;
    extern.loadLocalFileInEditor = loadLocalFileInEditor;
    extern.saveEditorToLocalFile = saveEditorToLocalFile;
    extern.loadExample = loadExample;
    extern.saveToDropbox = saveToDropbox;
    extern.loadFromDropbox = loadFromDropbox;



    var controller = null;

    // Make the console controller.
    function makeController(){
        controller = $('.mosml-console').console({
            promptLabel: '\u200b',   // zero width space, hack.
            commandHandle: function(line){
                wsSendCommand(line);
                return '\n';
            },
            autofocus: false,
            animateScroll: true,
            promptHistory: true,
            fadeOnReset: false
            //        welcomeMessage: 'Type SML in here.',
            //        continuedPromptLabel: ''
        });
    };

    function consoleReport(kind, msg) {
        controller.report([{msg: msg, className: kind}]);
    };

    function consoleError(msg) {
	    consoleReport('trymosml-error', msg);
    };

    function consoleInfo(msg) {
	    consoleReport('trymosml-info', msg);
    };

    function consoleWrite(msg) {
	    consoleReport('mosml-output', msg);
    };

    // The mosml websocket
    var ws = null;

    // Send the given line to the server via the ws websocket.
    function wsSendCommand(line) {
	    if (ws) {
		    try {
			    ws.send(line);
		    } catch (ex) {
			    consoleError('Cannot send: ' + ex);
		    }
	    } else {
		    consoleError('Cannot send: not connected to server');
	    }
    };

    function webSocketUrl() {
        var re = new RegExp('^(http)(s)?://([^/]*)(/.*)$');
        var match = location.href.match(re);
        if (match) {
            var wsUrl = 'ws' + (match[2] || '') + '://' + match[3] + match[4] + "webshell";
            return wsUrl;
        } else {
            return 'ws://try.mosml.org/webshell';
        }
    }

    function connectWebsocket () {
        var url = webSocketUrl();
        $(".run-code").prop('disabled', true);
	    try {
		    ws = new WebSocket(url);
	    } catch (ex) {
		    consoleError('Cannot connect to: ' + url + ' got the error '+ex);
		    ws = null;
		    return;
	    }

        var everConnected = false;
        var ready = false;
        var startupOutput = '';
        var readyMarker = '__trymosml_ready_' + Date.now() + '_' +
            Math.random().toString(36).slice(2) + '__';

	    ws.onopen = function(ev) {
            everConnected = true;
		    consoleInfo('Connected to Moscow ML server');
            initialiseToplevel(readyMarker);
	    };
	    ws.onclose = function(ev) {
		if (!ready && startupOutput) consoleWrite(startupOutput);
		consoleInfo(ev.reason || 'Connection closed to Moscow ML server');
            $(".run-code").prop('disabled', true);
		    ws = null;
	    };
	    ws.onmessage = function(ev) {
            if (!ready) {
                // A WebSocket can open before Docker or the REPL is ready.
                // Accumulate output because the marker can span frames.
                startupOutput += ev.data;
                var markerAt = startupOutput.indexOf(readyMarker);
                if (markerAt < 0) return;
                ready = true;
                $(".run-code").prop('disabled', false);
                consoleWrite(startupOutput.slice(0, markerAt) +
                    startupOutput.slice(markerAt + readyMarker.length));
                startupOutput = '';
            } else {
                consoleWrite(ev.data);
            }
	    };
	    ws.onerror = function(ev) {
		    consoleError('Connection error');
            if (! everConnected) {
                consoleError('Never managed to connect to Moscow ML server');
            }
	    };
    };

    var getAndUseName = "getAndUse";

    function initialiseToplevel(readyMarker) {
        var getAndUseDecl = ["fun ",getAndUseName," filename content = ",
                             "let val f = TextIO.openOut filename ",
                             "in  TextIO.output(f, content) ",
                             "  ; TextIO.closeOut f ",
                             "  ; use filename ",
                             "end;"].join('');
        wsSendCommand("Meta.quietdec := true;");
        wsSendCommand(getAndUseDecl);
        wsSendCommand("Meta.quietdec := false;");
        wsSendCommand('print "' + readyMarker + '";\n');
    }

    function getBufferName() {
        return $(".buffer-name").text();
    }

    function setBufferName(newName) {
        $(".buffer-name").text(newName);
    }

    function sendEditorContent() {
        var content = editor.getValue();
        var fname = getBufferName();
        wsSendCommand(getAndUseName + ' ' + smlString(fname) + ' ' +
            smlString(content) + ';\n');
    };

    // Moscow ML strings contain bytes. Quote UTF-8 with SML's three-digit
    // decimal escapes, so filenames and source travel in one complete command.
    function smlString(value) {
        return '"' + Array.from(new TextEncoder().encode(value), function (byte) {
            if (byte >= 32 && byte <= 126 && byte !== 34 && byte !== 92) {
                return String.fromCharCode(byte);
            }
            return '\\' + ('000' + byte).slice(-3);
        }).join('') + '"';
    }

    function resetConsole () {
        if (ws) {
            var oldClose = ws.onclose;
            ws.onclose = function (ev) {
                if(oldClose) oldClose(ev);
                controller.reset();
                connectWebsocket();
            };
            ws.close();
        } else {
            controller.reset();
            connectWebsocket();
        }
    };

    var editor = null;

    function makeEditor () {
        var scratchpad = $('.scratchpad')[0];
        editor = CodeMirror.fromTextArea(scratchpad, {
            mode: "text/x-sml",
            autofocus: true,
            lineNumbers: true,
            extraKeys: {
                "Ctrl-B": sendEditorContent
            }
        });
    };

    function newFile(ev) {
        ev.preventDefault();

        var filename = prompt("Name for new file", "mycode.sml");
        if (filename) {
            // TODO: check that we have a valid filename
            editor.setValue('');
            setBufferName(filename);
            editor.focus();
        }
    };

    function loadLocalFileInEditor(evt) {
        var file = evt.target.files[0];

        if (file) {
            var reader = new FileReader();
            reader.onload = function(e) {
	            var content = e.target.result;
                setBufferName(file.name);
                editor.setValue(content);
                editor.focus();
            }
            reader.readAsText(file);
        } else {
            alert("Failed to load file");
        }
    };


    function saveEditorToLocalFile(ev) {
        ev.preventDefault();
        var content = editor.getValue();
        var textFileAsBlob = new Blob([content], {type:'text/plain'});
        var fileNameToSaveAs = getBufferName();

        var downloadLink = document.createElement('a');
        downloadLink.download = fileNameToSaveAs;
        downloadLink.href = URL.createObjectURL(textFileAsBlob);
        downloadLink.style.display = 'none';
        downloadLink.onclick = function (event) {
            document.body.removeChild(event.target);
        };
        document.body.appendChild(downloadLink);
        downloadLink.click();
        editor.focus();
    };

    function loadExample(file) {
        $.get("examples/"+file, function(content) {
            setBufferName(file);
            editor.setValue(content);
            editor.focus();
        }, "text");
        return false;
    }

    function saveToDropbox(ev) {
        ev.preventDefault();
        var content = editor.getValue();
        var fileNameToSaveAs = getBufferName();

        var dataUrl = "data:text/plain;base64," + btoa(content);
        Dropbox.save(dataUrl, fileNameToSaveAs);
    };

    function loadFromDropbox(ev) {
        ev.preventDefault();
        function success (files) {
            var file = files[0];
            $.get(file.link, function(content) {
                setBufferName(file.name);
                editor.setValue(content);
                editor.focus();
            });
        }
        Dropbox.choose({success: success,
                        linkType: "direct",
                        multiselect: false});
    };

    return extern;
}();


// Main entry point.
$(function(){
    $(".run-code").prop('disabled', true);
    $(".run-code").click(trymosml.sendEditorContent);
    $(".reset-console").click(trymosml.resetConsole);


    $("#file-browser").on("change", trymosml.loadLocalFileInEditor);
    $(".open-file").click(function(ev) {
        ev.preventDefault();
        $("#file-browser").click();

    });
    $(".save-file").click(trymosml.saveEditorToLocalFile);
    $(".new-file").click(trymosml.newFile);
    $(".load-example").click(function (ev) {
        ev.preventDefault();
        trymosml.loadExample($(ev.target).text());
    });
    $(".choose-dropbox").click(trymosml.loadFromDropbox);
    $(".save-dropbox").click(trymosml.saveToDropbox);

    trymosml.makeController();
    trymosml.connectWebsocket();
    trymosml.makeEditor();

});
