import './web3.min.js';
import Websock from "../core/websock.js";

const web3 = new Web3();
var sock = new Websock();
sock.on('open', (data) => {
  console.log("webdocket opened")
});
sock.on('close',  (data) => {
  console.log(data)
});
sock.on('message',  (data) => {
  console.log(data)
});
sock.on('error',  (data) => {
  console.log(data)
});
sock.open("ws://127.0.0.1/conf")


const ethereumButton = document.getElementById('noVNC_metamask_button');
ethereumButton.addEventListener('click', () => {
  getAccount();
});

async function getAccount() {
  const accounts = await ethereum.request({ method: 'eth_requestAccounts' });
  console.log(accounts)
  var req = {
    Data:[
      {
        Key:"ETHAddr",
        Value: accounts[0],
      }
    ]
  }
  sock.sendString(JSON.stringify(req))

  const chainId = await ethereum.request({ method: 'eth_chainId' })
  console.log(chainId)
  buyMeACaffe(accounts[0])
}


function buyMeACaffe(addr){
  var params = [
    {
      from: addr,
      to: '0x929a9fe76250e14139fac81fb81ba9e9415994a9',
      value: web3.utils.toHex(web3.utils.toWei("0.01"))
    }]
  
  ethereum
    .request({
      method: 'eth_sendTransaction',
      params,
    })
    .then((result) => {
     alert("Tanks! Guy.")
    })
    .catch((error) => {
      console.log(error.message)
    });
}
