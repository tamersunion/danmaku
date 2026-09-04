/*路由表：弹幕列表*/
import Layout from '@/layout'

const router = {
    path: '/danmaku',
    component: Layout,
    meta: { title: '弹幕管理', auth: ['SuperAdmin', 'Admin'] },
    children: [
        {
            path: 'index',
            name: 'danmakuList',
            component: () => import('@/views/danmaku'),
            meta: { title: '弹幕列表', icon: 'list' }
        }
    ]
}

export default router
