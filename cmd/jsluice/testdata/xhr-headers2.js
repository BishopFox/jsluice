function insert(){
    if(window.XMLHttpRequest){
        xmlhttp = new XMLHttpRequest();
    }else{
        xmlhttp = new ActiveXObject('Microsoft.XMLHTTP');
    };

    xmlhttp.onreadystatechange = function(){
        if(xmlhttp.readyState == 4 && xmlhttp.status == 200){ };  
    };

    parameters = 'insert_text='+document.getElementById('insert_text').value;

    xmlhttp.open('POST','ajax_posting_data.php',true)

    console.log("Some stuff goes here")
    xmlhttp.setRequestHeader('Content-type', 'application/x-www-form-urlencoded');
    
    if (someThingIsTrue()){
        // Another header
        xmlhttp.setRequestHeader('X-Env', 'staging')
    }
    xmlhttp.send(parameters);
};
