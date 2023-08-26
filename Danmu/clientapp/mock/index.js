const { success } = require('./util')
const base = require('../src/config').apiPrefix

const routes = [
    { url: '/admin/login', method: 'post', result: require('./data/login') },
    { url: '/admin/danmakulist/vids', method: 'get', result: require('./data/vids') },
    { url: '/admin/danmakulist/baseselect', method: 'get', result: require('./data/danmakuList') },
    { url: '/admin/danmakuedit/delete', method: 'get', result: null },
    { url: '/admin/danmakuedit', method: 'get', result: require('./data/danmakuInfo') },
    { url: '/admin/danmakuedit/edit', method: 'post', result: null },
    { url: '/admin/user/changepassword', method: 'post', result: null },
    { url: '/admin/user/changeinfo', method: 'post', result: null },
    { url: '/admin/user/user', method: 'get', result: require('./data/userInfo') }
]

module.exports = app => routes.forEach(route =>
    app[route.method](base + route.url, (req, res) => {
        const result = typeof route.result === 'function' ? route.result(req, res) : route.result
        res.send(success(result))
    }))
