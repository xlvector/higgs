var id = "";
var publicKey = "";
var pki = forge.pki;
var start = (new Date()).getTime();
var userid = "";
var tmpl = "";

function printTime(result) {
    var end = (new Date()).getTime();
    $(result).append((end - start) / 1000 + "秒<br/>");
    start = (new Date()).getTime();
}

function getData(param) {
    $.ajax({
        url: "../submit?" + $.param(param),
        dataType: "json",
        success: function(data){
            id = data.id;
            if (data.status == "output_qrcode"){
                $('#result').empty();
                $('#result').html("");
                $("#result").append("<img id=\"randcode_img\" src='" + data.data +"'/><br/>");
                getData({tmpl: tmpl, id: data.id, t2: (new Date()).getTime()});
            } else if (data.status == "login_success"){
                $("#result").append("登录成功<br/>");
                getData({tmpl: tmpl, id: data.id});
            } else if (data.status == "fail") {
                $("#result").append("失败: " + data.data + "<br/>");
            } else if (data.status == "finish_fetch_data") {
                $("#result").append("成功获取数据<br/>");
                printTime();
            }
        },
        error: function() {
            $("#result").append("超时");
            printTime();
        },
        timeout: 120000,
    });
}

function crawl() {
    start = (new Date()).getTime();
    tmpl = $("#tmpl").val();

    if (tmpl.length == 0) {
        alert("您的输入有误，请检查！");
        return;
    }
    $("#result").html("开始<br/>");
    getData({tmpl: tmpl,t1: (new Date()).getTime()});
}
