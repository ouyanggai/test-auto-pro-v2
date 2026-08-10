/*
 * @Descripttion:
 * @Author: Calvin
 * @Date: 2021-01-14 11:47:57
 */
import Vue from 'vue';
import SvgIcon from '@/components/SvgIcon';// svg component

// register globally
Vue.component('SvgIcon', SvgIcon);

const req = require.context('./svg', false, /\.svg$/);
const requireAll = requireContext => requireContext.keys().map(requireContext);
requireAll(req);
