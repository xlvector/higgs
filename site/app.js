var id = "";
var publicKey = "";
var pki = forge.pki;
var start = (new Date()).getTime();
var username = "";
var password = "";
var userid = "";
var tmpl = "";

function printTime() {
    var end = (new Date()).getTime();
    $("#result").append((end - start) / 1000 + "秒<br/>");
    start = (new Date()).getTime();
}

function getData(param) {
    $.ajax({
        url: "../submit?" + $.param(param),
        dataType: "json",
        success: function(data){
            id = data.id;
            printTime();
            if (data.status == "output_verifycode"){
                $('#result').empty();
                $('#result').html("");
                $("#result").append("<img id=\"randcode_img\" src='" + data.data +"'/>");
                var input = "<div class=\"form-group\">" +
                    "<label for=\"password\">验证码</label>" +
                    "<input type=\"txt\" class=\"form-control\" id=\"randcode_input\" placeholder=\"验证码\"></div>";
                $("#result").append(input);
                var button = "<div class=\"form-group\">"+
                    "<button type=\"submit\" onclick=\"sendRandcode();\" class=\"btn btn-default\" id=\"randcode_send\">发送验证码</button></div>";
                $("#result").append(button);
            }
            else if(data.status == "need_param") {
                if (data.need_param == "password2"){
                    $("#result").empty();
                    $("#result").html("");
                    if (data.data.length != 0) {
                        $("#result").append("手机号码："+data.data+"<br/>");
                    }
                    addPassword2Div("#result");
                }

                if (data.need_param == "phone") {
                    $("#result").empty();
                    $("#result").html("");
                    var phones = jQuery.parseJSON(data.data);

                    var phoneSelect = "<div class=\"form-group\"><select id=\"phone\">";
                    $.each(phones,function(index, value){
                        phoneSelect = phoneSelect + "<option value=\""+value+"\">"+value+"</option>"
                    });
                    phoneSelect = phoneSelect+"</select></div>";
                    $("#result").append(phoneSelect);

                    var button = "<div class=\"form-group\">"+
                        "<button type=\"submit\" onclick=\"sendPhone();\" class=\"btn btn-default\" id=\"randcode_send\">"+
                        "发送手机号码</button></div>";
                    $("#result").append(button);
                }
            } else if (data.status == "fail") {
                if (tmpl == "taobao_shop" || tmpl == "tmall_shop") {
                    alert("抓取失败:"+data.data);
                }
            } else if (data.status == "login_success") {
                $("#result").append("登录成功<br/>");
                getData({id: data.id});
            } else if (data.status == "begin_fetch_data") {
                $("#result").append("开始获取数据<br/>");
                getData({id: data.id});
            } else if (data.status == "finish_fetch_data") {
                $("#result").append("成功获取数据<br/>");
            } else if(data.status == "output_publickey") {
                publicKey = pki.publicKeyFromPem(data.data);
                var encrypted = publicKey.encrypt(password, 'RSA-OAEP', {md: forge.md.sha256.create()});
                encrypted = forge.util.binary.hex.encode(encrypted);
                getData({id: data.id, username: username, password: encrypted})
                $("#result").append("获取公钥成功<br/>");
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
    username = $("#username").val();
    password = $("#password").val();
    userid = $("#userid").val();
    start = (new Date()).getTime();
    tmpl = $("#tmpl").val();

    if (username.length == 0 || password.length == 0 || tmpl.length == 0) {
        alert("您的输入有误，请检查！");
        return;
    }
    $("#result").html("开始<br/>");
    getData({tmpl: tmpl,userid: userid});
}

function addPassword2Div(container) {
    $(container).append('<div id="password2"></div>')
    $("#password2").append('<div class="form-group">'
            + '<input type="password" class="form-control" id="password2_input" placeholder="独立密码" />'
            + '</div>');
    $("#password2").append('<div class="form-group">'
            + '<button type="submit" onclick="sendPassword2();" class="btn btn-default">提交独立密码</button>'
            + '</div>');
}

function sendPassword2() {
    var password2 = $("#password2_input").val();
    if (id.length == 0 || password2.length == 0) {
        alert("您的输入有误，请检查！");
        return;
    }

    getData({id: id, password2: password2});
}

function sendPhone() {
    alert("start send phone");
    var phone = $("#phone").val();
    if (id.length == 0 || phone.length == 0) {
        alert("您的输入有误，请检查！");
        return;
    }
    getData({id: id, phone: phone});
}

function sendRandcode() {
    var randcode = $("#randcode_input").val();
    if (id.length == 0 || randcode.length ==0) {
        alert("您的输入有误，请检查！");
        return;
    }
    getData({id: id, randcode: randcode});
}


