(function () {
    'use strict';

    var series = [
        ['all', '全部'],
        ['oriental', '东方器韵'],
        ['masters', '艺术史'],
        ['nature', '自然地貌'],
        ['cinema', '电影光色'],
        ['material', '建筑材质'],
        ['editorial', '编辑印刷'],
        ['digital', '数字未来'],
        ['subculture', '亚文化'],
        ['craft', '奢华工艺'],
        ['quiet', '极简生活']
    ];

    var themeRows = [
        ['001', '青瓷雨', 'Celadon Rain', 'oriental', ['#E7EFEA', '#B8D2C7', '#7FA99A', '#3F6F66', '#283C39'], '雨水洗过青瓷釉面，温润、克制，带一线松石冷光。'],
        ['002', '朱砂雪', 'Cinnabar Snow', 'oriental', ['#F5F1E9', '#D8D0C3', '#C64736', '#812A22', '#251D1A'], '宣纸留白与朱砂印记相遇，清峻之中保留温度。'],
        ['003', '汝窑天青', 'Ru Sky', 'oriental', ['#EEF2ED', '#C9D9D1', '#9EB9AF', '#66877E', '#3F514D'], '取雨过天青的轻灰绿，像薄釉下静止的一层雾。'],
        ['004', '敦煌暮色', 'Dunhuang Dusk', 'oriental', ['#2B2533', '#66506B', '#B56A4C', '#D9A441', '#E7D2A1'], '石窟壁画褪色后的紫、土红与矿物金，古老而灿烂。'],
        ['005', '宋锦夜航', 'Song Brocade Night', 'oriental', ['#101F2B', '#234354', '#8C3342', '#D1A250', '#E8DDC8'], '深靛织锦上穿过一叶金色小舟，端庄又有隐秘华光。'],
        ['006', '漆金残月', 'Lacquer Moon', 'oriental', ['#171312', '#3B2420', '#8E3329', '#C8953D', '#F0DCA8'], '黑漆、暗朱与旧金箔构成残月般的东方戏剧性。'],
        ['007', '松烟水墨', 'Pine Soot Ink', 'oriental', ['#F2F0E9', '#C9C8C0', '#888B86', '#414743', '#161B19'], '五级墨色从纸白缓慢沉入松烟，适合安静而坚定的表达。'],
        ['008', '和纸樱影', 'Washi Sakura', 'oriental', ['#FAF4EE', '#EBCFD0', '#D79EA7', '#8F6A71', '#463B3D'], '纤维纸、落樱与茶灰色阴影，轻盈但不甜腻。'],
        ['009', '浮世潮汐', 'Ukiyo Tide', 'oriental', ['#F1E4C8', '#D06B46', '#274D66', '#173342', '#92B9B2'], '木版海浪的靛蓝、浅葱与赭红，节奏鲜明。'],
        ['010', '高丽青黛', 'Goryeo Indigo', 'oriental', ['#E4E8DC', '#A8BFAF', '#536F6A', '#243D4A', '#172531'], '青瓷绿向深海靛过渡，古典器物感里藏着冷峻现代性。'],

        ['011', '波提切利花园', 'Botticelli Garden', 'masters', ['#EBDCC2', '#C7B48A', '#88936B', '#A75E58', '#5A4140'], '文艺复兴花园里的柔金、鼠尾草与旧玫瑰。'],
        ['012', '维米尔之窗', 'Vermeer Window', 'masters', ['#E9D9A9', '#C5A34A', '#2F5B70', '#1D3443', '#6B5040'], '侧窗冷光照亮铅锡黄与群青，沉静、精确、耐看。'],
        ['013', '克里姆特金吻', 'Gilded Embrace', 'masters', ['#17150F', '#57451D', '#B58B2A', '#E0C063', '#A7523F'], '深色背景托起层叠金箔与一点砖红，华丽却有重量。'],
        ['014', '马蒂斯剪纸', 'Cut Paper Joy', 'masters', ['#F5EBDD', '#EE6B3B', '#315FA4', '#15967D', '#F2C84B'], '高纯度色块像剪纸般碰撞，坦率、明亮、充满生命力。'],
        ['015', '莫兰迪静物', 'Morandi Silence', 'masters', ['#D8D0C3', '#B8AA9D', '#938B82', '#7F938B', '#6E6460'], '灰粉、瓶绿与陶土褐降低音量，留下长久的平衡。'],
        ['016', '罗斯科余烬', 'Rothko Ember', 'masters', ['#241922', '#512330', '#9A3B32', '#D16A3D', '#D9A66D'], '色域像余烬从暗紫缓慢燃向橙褐，具有冥想般的深度。'],
        ['017', '北斋浪尖', 'Cresting Wave', 'masters', ['#F2E7CF', '#B8D0CA', '#507B91', '#204D6B', '#142E46'], '海沫白与普鲁士蓝切出凌厉浪形，清澈而有力量。'],
        ['018', '包豪斯原点', 'Bauhaus Origin', 'masters', ['#F3EFE4', '#20242A', '#D84936', '#E6B936', '#2D66A3'], '黑色骨架与红黄蓝三原色，理性比例中保留游戏感。'],
        ['019', '新艺术鸢尾', 'Nouveau Iris', 'masters', ['#EEE6D2', '#9EAE89', '#586D5D', '#76618D', '#C690A8'], '植物曲线、鸢尾紫与柔和苔绿，优雅且具有装饰张力。'],
        ['020', '印象派正午', 'Impressionist Noon', 'masters', ['#F4E9B8', '#D8C85C', '#78A6B8', '#587E9C', '#B98490'], '阳光碎片落在水面，蓝、黄与淡紫以短促笔触闪烁。'],

        ['021', '极地晨曦', 'Polar Dawn', 'nature', ['#F5F8F7', '#CDE0E2', '#91BCC8', '#827AA8', '#E6A5A7'], '冰原尽头升起粉紫晨光，清冷中带一丝柔软。'],
        ['022', '火山苔原', 'Volcanic Moss', 'nature', ['#191B19', '#373C32', '#687352', '#B3593D', '#D99854'], '玄武岩、湿苔与熔岩余温构成原始而粗粝的生命感。'],
        ['023', '盐湖镜面', 'Salt Mirror', 'nature', ['#F6F2E9', '#D9E4DE', '#A7CDCA', '#DDA7A1', '#8A7D86'], '盐白天空倒映浅青与霞粉，空气近乎透明。'],
        ['024', '沙丘月影', 'Dune Moon', 'nature', ['#171A22', '#343747', '#8B725B', '#C39A69', '#E2C79D'], '夜蓝压住起伏沙脊，月光把边缘磨成古铜色。'],
        ['025', '雨林脉搏', 'Rainforest Pulse', 'nature', ['#0D2822', '#17523E', '#3E7A4D', '#A1B84B', '#E26A45'], '层叠叶绿之间跳出一枚热带橙红，浓郁且湿润。'],
        ['026', '冰川裂隙', 'Glacier Rift', 'nature', ['#E9F2F3', '#A9D5DF', '#55A3BD', '#226A8A', '#16384E'], '从霜白滑向裂隙深蓝，锋利、洁净、具有纵深。'],
        ['027', '荒漠星图', 'Desert Constellation', 'nature', ['#111927', '#23304A', '#8B6650', '#D1A26B', '#F0D7A6'], '深夜沙漠的群青与金砂，适合辽阔而神秘的叙事。'],
        ['028', '珊瑚浅海', 'Coral Lagoon', 'nature', ['#EAF3EC', '#87C8BE', '#369FA0', '#F17D68', '#D95050'], '浅海薄荷蓝映着珊瑚红，明快但不过分稚嫩。'],
        ['029', '高山杜鹃', 'Alpine Rhododendron', 'nature', ['#EDF0E7', '#A9B590', '#596B58', '#B34D67', '#6A304C'], '岩灰绿地上盛开浓艳杜鹃，克制背景托起高海拔花色。'],
        ['030', '深海磷光', 'Abyssal Glow', 'nature', ['#050C18', '#0C2339', '#13506B', '#20B2A4', '#A7F2C4'], '深海蓝黑中浮出微弱绿光，安静、未来且不可测。'],

        ['031', '王家卫雨夜', 'Neon Rain', 'cinema', ['#0E1D23', '#164B4B', '#B1293F', '#E45B39', '#E7B05A'], '湿街、红灯与青绿阴影，像一段被拉长的城市记忆。'],
        ['032', '法式午后', 'French Matinee', 'cinema', ['#F2E8D5', '#D7C39F', '#86A0A2', '#C66B65', '#543F43'], '褪色胶片里的奶油阳光、雾蓝与克制唇红。'],
        ['033', '西部落日', 'Western Sundown', 'cinema', ['#2B1E20', '#6F3330', '#B95D38', '#E39A4C', '#E7CA8E'], '尘土、皮革与燃烧天空，粗犷而有史诗感。'],
        ['034', '银翼迷城', 'Replicant City', 'cinema', ['#090D15', '#162238', '#22516C', '#D34530', '#E7B84B'], '冷蓝高楼、雨幕与远处红黄霓虹，压迫又迷人。'],
        ['035', '樱桃旅馆', 'Cherry Motel', 'cinema', ['#231C2B', '#5F345C', '#C84D72', '#ED8A8C', '#F3C9A9'], '旧旅馆的紫红招牌在粉橙夜色里闪烁，甜美而危险。'],
        ['036', '北欧悬疑', 'Nordic Noir', 'cinema', ['#111619', '#273237', '#526168', '#8A9995', '#D4D6CD'], '低饱和海岸、铅灰天空与冷白雾气，理性而疏离。'],
        ['037', '默片金尘', 'Silent Film Gold', 'cinema', ['#171512', '#3D382F', '#81715B', '#C4A978', '#E8D9B8'], '黑白胶片被岁月染成棕金，庄重、怀旧、有颗粒感。'],
        ['038', '夏日公路', 'Summer Road', 'cinema', ['#F6D98F', '#E69B55', '#D65C49', '#4D8391', '#244B5B'], '正午黄、汽车旅馆橙与远方公路蓝，明亮而自由。'],
        ['039', '月球静海', 'Lunar Silence', 'cinema', ['#090A0D', '#24272E', '#555C68', '#AEB5BC', '#E8E9E5'], '黑色宇宙托住银灰月尘，极简画面里保留宏大尺度。'],
        ['040', '歌舞厅午夜', 'Cabaret Midnight', 'cinema', ['#160E19', '#3D1834', '#842F58', '#D15965', '#E6BD75'], '天鹅绒紫、舞台红与香槟金，浓烈而不失精致。'],

        ['041', '清水混凝土', 'Quiet Concrete', 'material', ['#EEEDE9', '#C9C9C4', '#999D9B', '#5E6665', '#2F3535'], '从水泥白到结构灰，冷静比例让材质本身成为主角。'],
        ['042', '氧化铜墙', 'Verdigris Wall', 'material', ['#182825', '#31564E', '#5E8878', '#B26745', '#D1A06B'], '铜绿与锈橙在旧墙面交错，时间感浓厚。'],
        ['043', '洞石午光', 'Travertine Light', 'material', ['#F0E9DD', '#D8C9B4', '#B8A286', '#806E5C', '#51463D'], '洞石孔隙里的暖白与砂褐，安静、坚实、触感清晰。'],
        ['044', '黑钢与火', 'Black Steel', 'material', ['#101214', '#292E33', '#535A61', '#C14D32', '#E09A4A'], '哑黑钢板被一线炉火点亮，工业感直接而有温度。'],
        ['045', '彩窗教堂', 'Stained Cathedral', 'material', ['#11182A', '#274F8A', '#8B315D', '#D69D35', '#E8D59D'], '深色石墙框住宝石蓝、玫瑰紫与圣像金。'],
        ['046', '红砖雨巷', 'Rainwashed Brick', 'material', ['#E6DED2', '#A97860', '#8B4A3B', '#4F5C59', '#283634'], '湿润红砖与深灰绿门框，朴素却具有生活质地。'],
        ['047', '铝与钴蓝', 'Aluminium Cobalt', 'material', ['#EEF0F2', '#BFC6CE', '#7C8794', '#3156A3', '#172D59'], '金属银灰承托纯净钴蓝，像精密仪器般清醒。'],
        ['048', '陶土拱廊', 'Terracotta Arcade', 'material', ['#F0E3D4', '#D6AA82', '#B66B4B', '#7D4435', '#324B48'], '陶土墙、拱形阴影与一点墨绿，具有南方建筑的呼吸。'],
        ['049', '玻璃薄雾', 'Frosted Glass', 'material', ['#F5F7F6', '#DDE5E5', '#B9CED0', '#789A9F', '#536A72'], '磨砂玻璃把光分解成五级冷灰蓝，透明又私密。'],
        ['050', '黄铜电梯', 'Brass Elevator', 'material', ['#171714', '#383526', '#71633B', '#B58A44', '#E2C883'], '深色电梯厅里的拉丝黄铜，克制地表达奢华。'],

        ['051', '瑞士网格', 'Swiss Grid', 'editorial', ['#F5F4EF', '#1A1C1F', '#D9382A', '#B8BDC2', '#6C7178'], '严格网格、无衬线黑字与信号红，清晰即是美感。'],
        ['052', '午夜报刊', 'Midnight Gazette', 'editorial', ['#111317', '#262B31', '#D9D3C5', '#9E3036', '#C4A45B'], '黑底报纸、骨白正文与暗红标题，信息密度充满戏剧性。'],
        ['053', '蓝图档案', 'Blueprint Archive', 'editorial', ['#E9EDF0', '#AAC2D4', '#456E91', '#1F476B', '#142E49'], '工程蓝图与档案编号构成理性、可信的视觉语言。'],
        ['054', '文学季刊', 'Literary Quarterly', 'editorial', ['#F0EBDD', '#B8AE99', '#5C554D', '#873E39', '#2B2825'], '纸张、墨色与一枚暗红书脊，适合长阅读与思想内容。'],
        ['055', '新潮杂志', 'New Wave Magazine', 'editorial', ['#F2F0EA', '#16191D', '#6A4BC3', '#F06445', '#B8D74E'], '锐利标题与意外撞色，年轻、实验、不讨好。'],
        ['056', '科学年鉴', 'Science Annual', 'editorial', ['#F4F6F3', '#CBD8D4', '#44766C', '#D3973C', '#303C42'], '实验室绿、索引黄与石墨字色，准确中带人文气息。'],
        ['057', '诗歌小册', 'Poetry Pamphlet', 'editorial', ['#F7F3EC', '#DDD3C5', '#8A7D76', '#A45D66', '#4A4146'], '大量留白与低声的灰玫瑰，像一句没有说完的诗。'],
        ['058', '交通导视', 'Transit Manual', 'editorial', ['#F0F1EE', '#24272B', '#176C91', '#E6AA2F', '#C44936'], '公共系统的蓝、黄、红被精确编排，直接且友好。'],
        ['059', '唱片内页', 'Liner Notes', 'editorial', ['#E7DED0', '#1D1B1A', '#66544A', '#B43D32', '#D7A74D'], '粗粝纸张、黑胶黑与巡演海报红，带着模拟时代的温度。'],
        ['060', '未来目录', 'Future Catalogue', 'editorial', ['#E8EBEE', '#ADB8C1', '#354451', '#235DD8', '#E15338'], '冷银版面中插入高纯蓝与橙红，像尚未发行的设计年鉴。'],

        ['061', '液态铬', 'Liquid Chrome', 'digital', ['#080B10', '#222935', '#667589', '#B9D1E6', '#7B5CFF'], '深色界面、液态银与电紫反光，具有高精度未来感。'],
        ['062', '生物荧光', 'Bio Lumina', 'digital', ['#061711', '#0B382C', '#11725A', '#43D18C', '#C1FF72'], '有机深绿里生长出荧光细胞，科技却不冰冷。'],
        ['063', '量子珊瑚', 'Quantum Coral', 'digital', ['#0E1020', '#252A55', '#6A4FB3', '#E45574', '#FF9B73'], '深蓝计算空间里漂浮珊瑚粉粒子，柔软而前沿。'],
        ['064', '数据冰川', 'Data Glacier', 'digital', ['#07131E', '#123653', '#19769B', '#57C7D4', '#D1FAF5'], '数据流像冰川裂面反射青光，洁净、可信、快速。'],
        ['065', '合成日落', 'Synthetic Sunset', 'digital', ['#17102B', '#472D7B', '#A8448A', '#ED6B5B', '#F4C45D'], '紫色天际线向人工橙黄过渡，怀旧未来主义的经典气候。'],
        ['066', '矩阵花园', 'Matrix Garden', 'digital', ['#07120D', '#173C2A', '#2B7750', '#78D464', '#D6F06C'], '代码绿从暗色土壤中抽枝，像一座可计算的植物园。'],
        ['067', '全息薄荷', 'Hologram Mint', 'digital', ['#F2F7F5', '#C0E8DD', '#7BCDC5', '#9B8DE3', '#F0A4C5'], '薄荷青、全息紫与珍珠粉在浅色表面轻轻偏移。'],
        ['068', '红色协议', 'Red Protocol', 'digital', ['#0D0E10', '#282B30', '#6A1F25', '#D92F3D', '#FF6A4D'], '安全警报般的高能红色被黑色结构严格约束。'],
        ['069', '轨道黎明', 'Orbital Dawn', 'digital', ['#090D1C', '#1D2D55', '#315F9A', '#E47B55', '#F5C982'], '星球边缘的第一束暖光切开轨道深蓝。'],
        ['070', '人工海风', 'Artificial Sea', 'digital', ['#EAF5F3', '#9EDAD2', '#3FAFAE', '#256D8B', '#213A5D'], '数字海面从泡沫白滑向深蓝，清爽且具有交互感。'],

        ['071', '蒸汽波商场', 'Vapor Mall', 'subculture', ['#15142A', '#3F3371', '#A94FA4', '#F486B5', '#63D6D2'], '夜间商场、棕榈剪影与粉青霓虹，甜美又疏离。'],
        ['072', '酸性俱乐部', 'Acid Club', 'subculture', ['#0B0C0B', '#292D22', '#98E22B', '#F2EE3A', '#E84F9A'], '酸绿、荧光黄与品红在黑暗舞池里剧烈跳动。'],
        ['073', '滑板录像带', 'Skate Tape', 'subculture', ['#E5DCCB', '#22211F', '#417B79', '#D05239', '#E7B845'], '磨损磁带、混凝土与贴纸撞色，粗糙但真诚。'],
        ['074', '朋克传单', 'Punk Xerox', 'subculture', ['#F1EEE5', '#171717', '#525252', '#D82932', '#E8E33A'], '复印黑、撕纸白、警告红黄，拒绝精致的高能秩序。'],
        ['075', '千禧糖壳', 'Y2K Candy', 'subculture', ['#EDF3F4', '#A9DCE0', '#BEA3EA', '#F49BC0', '#B7D65B'], '半透明塑料、糖果粉紫与青苹果绿，轻松而闪亮。'],
        ['076', '暗黑浪漫', 'Dark Romance', 'subculture', ['#100D11', '#2C1724', '#5D263D', '#9D3F57', '#C9A2A7'], '黑蕾丝、干玫瑰与酒红天鹅绒，阴郁中保留细腻。'],
        ['077', '电玩街机', 'Arcade Fever', 'subculture', ['#080A18', '#1E1C54', '#315BEA', '#F03FA6', '#F4D63B'], '蓝色像素、粉色激光与黄色得分牌，节奏快速。'],
        ['078', '沙漠嬉皮', 'Desert Nomad', 'subculture', ['#F0DFC0', '#D3985B', '#A5563D', '#576B58', '#303D3B'], '日晒棉布、陶土与仙人掌绿，松弛又有手作感。'],
        ['079', '哥特圣像', 'Gothic Icon', 'subculture', ['#0E0C10', '#28222C', '#56364E', '#A4854D', '#D3C4A2'], '黑色尖拱、紫灰阴影与旧金圣像，神秘且庄严。'],
        ['080', '原宿贴纸', 'Harajuku Stickers', 'subculture', ['#FFF2E8', '#FC7795', '#F7C943', '#56BDD0', '#7651B8'], '贴纸般的粉黄蓝紫并置，充满街头玩心。'],

        ['081', '孔雀珠宝盒', 'Peacock Jewel', 'craft', ['#0B2626', '#145B59', '#207E78', '#B08A3E', '#D4BB73'], '孔雀蓝绿与古金镶嵌，浓郁色泽像打开一只珠宝盒。'],
        ['082', '香槟丝绒', 'Champagne Velvet', 'craft', ['#241D21', '#59414A', '#947079', '#C9A66B', '#ECDDBB'], '烟粉丝绒与香槟金交叠，柔和、成熟、不张扬。'],
        ['083', '午夜蓝宝石', 'Midnight Sapphire', 'craft', ['#080E1A', '#132947', '#204F82', '#B08C49', '#E3D0A1'], '蓝宝石深色切面被细窄金边照亮，冷艳而精密。'],
        ['084', '翡翠烟盒', 'Jade Case', 'craft', ['#10251F', '#285344', '#55806A', '#B08A52', '#E0CFAB'], '深翡翠、黄铜与骨白内衬，带有旧时代的仪式感。'],
        ['085', '珍珠歌剧院', 'Pearl Opera', 'craft', ['#F2EEE8', '#D8CCD0', '#A88D98', '#593C4A', '#B9955A'], '珍珠母贝与剧院绛紫相遇，光泽细腻且富有层次。'],
        ['086', '琥珀香室', 'Amber Chamber', 'craft', ['#21140E', '#5C321B', '#A45D24', '#D79A3D', '#F0C977'], '蜂蜜、琥珀与深木色在烛光里逐层透亮。'],
        ['087', '银器蓝绸', 'Silver Silk', 'craft', ['#E8EBED', '#B9C0C7', '#6F7D8C', '#38516F', '#192B43'], '冷银器皿落在深蓝绸面，光影锐利又流动。'],
        ['088', '玳瑁与金', 'Tortoiseshell Gold', 'craft', ['#1C1410', '#4C2918', '#8E5126', '#C18A43', '#E2BF78'], '半透明棕褐纹理与暖金边框，复古、厚重、富有细节。'],
        ['089', '紫晶沙龙', 'Amethyst Salon', 'craft', ['#17121D', '#382843', '#674C75', '#9A79A0', '#C7A76A'], '紫晶色阶与一线古金，像夜晚私人沙龙的灯光。'],
        ['090', '黑瓷白金', 'Obsidian Platinum', 'craft', ['#090A0B', '#242629', '#555A60', '#AEB4B8', '#ECEBE7'], '黑瓷光面与白金冷辉，极简轮廓承载高级质感。'],

        ['091', '晨间亚麻', 'Morning Linen', 'quiet', ['#F4F1EB', '#DFD8CC', '#B8AF9F', '#7E8176', '#555B55'], '被晨光照亮的亚麻、陶器与一枝灰绿植物。'],
        ['092', '北窗白', 'North Window', 'quiet', ['#F6F7F5', '#E2E5E2', '#C2C9C6', '#89938F', '#505B58'], '北向窗户带来的均匀冷白，适合专注与长时间观看。'],
        ['093', '燕麦拿铁', 'Oat Latte', 'quiet', ['#F3EDE3', '#DDCFBA', '#BCA58A', '#826D5B', '#50443A'], '燕麦、浅木与咖啡褐组成柔软日常，没有多余装饰。'],
        ['094', '雨天书桌', 'Rainy Desk', 'quiet', ['#EEF0ED', '#C9D0CC', '#879691', '#576763', '#33413F'], '雨水把窗外降成灰绿色，桌面因此显得格外安静。'],
        ['095', '粉笔与木', 'Chalk and Oak', 'quiet', ['#F5F3EC', '#D7D2C5', '#B59A77', '#796650', '#41403B'], '粉笔白、橡木色与石墨灰，温和却有清晰结构。'],
        ['096', '鼠尾草厨房', 'Sage Kitchen', 'quiet', ['#F1F1E8', '#CED5C2', '#9FAE92', '#687962', '#454D43'], '雾面鼠尾草绿与奶白陶器，清新但不轻浮。'],
        ['097', '海盐浴室', 'Sea Salt', 'quiet', ['#F6F7F4', '#DDE7E4', '#B6D0CB', '#789F9B', '#4E6E6C'], '海盐白、浅水绿与湿润石材色，干净且放松。'],
        ['098', '无花果午睡', 'Fig Siesta', 'quiet', ['#F2EAE0', '#D3B9A9', '#A27675', '#67515D', '#44414A'], '午后阴影里的无花果紫与粉土色，慵懒而成熟。'],
        ['099', '冬日羊毛', 'Winter Wool', 'quiet', ['#F0EFEC', '#D2D0CB', '#AAA8A4', '#777A7B', '#4B5052'], '未经漂白的羊毛与冬日灰光，触感柔软、情绪稳定。'],
        ['100', '一页留白', 'A Blank Page', 'quiet', ['#FAFAF7', '#E9E8E2', '#C7C8C3', '#767B78', '#202523'], '以纸白开始，以墨黑结束；为内容保留最大呼吸空间。']
    ];

    var seriesNames = Object.fromEntries(series.map(function (item) { return [item[0], item[1]]; }));
    var themes = themeRows.map(function (row, index) {
        return {
            id: row[0],
            name: row[1],
            en: row[2],
            series: row[3],
            colors: row[4],
            description: row[5],
            sheet: Math.floor(index / 10) + 1,
            cell: index % 10,
            drift: ((index * 7) % 9) - 4
        };
    });

    var gallery = document.getElementById('theme-gallery');
    var filterRoot = document.getElementById('theme-filters');
    var searchInput = document.getElementById('theme-search');
    var countNode = document.getElementById('visible-count');
    var activeSeriesNode = document.getElementById('active-series');
    var emptyNode = document.getElementById('theme-empty');
    var dialog = document.getElementById('theme-dialog');
    var dialogArt = document.getElementById('dialog-art');
    var toast = document.getElementById('atlas-toast');
    var activeSeries = 'all';
    var toastTimer;

    var implementedThemes = {
        '001': 'atlas-celadon',
        '051': 'atlas-swiss',
        '061': 'atlas-chrome'
    };

    function colorVariables(colors) {
        return colors.map(function (color, index) {
            return '--c' + (index + 1) + ':' + color;
        }).join(';');
    }

    function symbolVariables(theme) {
        var column = theme.cell % 5;
        var row = Math.floor(theme.cell / 5);
        var sheet = String(theme.sheet).padStart(2, '0');
        return '--symbol-sheet:url(/images/visual-symbols/visual-symbols-' + sheet + '.webp);' +
            '--symbol-x:' + (column * 25) + '%;' +
            '--symbol-y:' + (row * 100) + '%;' +
            '--drift:' + theme.drift + 'px;';
    }

    function symbolMarkup(theme) {
        return '<span class="symbol-image" style="' + symbolVariables(theme) + '"></span>';
    }

    function filterThemes() {
        var query = searchInput.value.trim().toLocaleLowerCase('zh-CN');
        return themes.filter(function (theme) {
            var seriesMatch = activeSeries === 'all' || theme.series === activeSeries;
            var text = [theme.name, theme.en, seriesNames[theme.series], theme.description].join(' ').toLocaleLowerCase('zh-CN');
            return seriesMatch && (!query || text.includes(query));
        });
    }

    function renderGallery() {
        var visibleThemes = filterThemes();
        gallery.innerHTML = visibleThemes.map(function (theme) {
            var dots = theme.colors.map(function (color) {
                return '<i style="background:' + color + '" title="' + color + '"></i>';
            }).join('');
            return '<article class="specimen-card" role="button" tabindex="0" data-theme-id="' + theme.id + '" aria-label="查看 ' + theme.name + ' 配色详情" style="--drift:' + theme.drift + 'px">' +
                '<div class="specimen-art" aria-hidden="true">' + symbolMarkup(theme) + '</div>' +
                '<div class="specimen-meta">' +
                    '<div class="specimen-heading"><span>' + theme.id + '</span><div><h2>' + theme.name + '</h2><p>' + theme.en + '</p></div></div>' +
                    '<div class="specimen-palette" aria-hidden="true">' + dots + '</div>' +
                    '<div class="specimen-foot"><span>' + seriesNames[theme.series] + '</span><span>细看 ↗</span></div>' +
                '</div>' +
            '</article>';
        }).join('');

        countNode.textContent = String(visibleThemes.length).padStart(2, '0');
        activeSeriesNode.textContent = activeSeries === 'all' ? '全部馆藏' : seriesNames[activeSeries];
        emptyNode.hidden = visibleThemes.length !== 0;
    }

    function renderFilters() {
        filterRoot.innerHTML = series.map(function (item) {
            var pressed = item[0] === activeSeries;
            return '<button class="atlas-filter' + (pressed ? ' is-active' : '') + '" type="button" data-series="' + item[0] + '" aria-pressed="' + pressed + '">' + item[1] + '</button>';
        }).join('');
    }

    function showToast(message) {
        window.clearTimeout(toastTimer);
        toast.textContent = message;
        toast.classList.add('is-visible');
        toastTimer = window.setTimeout(function () {
            toast.classList.remove('is-visible');
        }, 1800);
    }

    function copyColor(color) {
        if (navigator.clipboard && window.isSecureContext) {
            navigator.clipboard.writeText(color).then(function () {
                showToast('已复制 ' + color);
            }).catch(function () { fallbackCopy(color); });
            return;
        }
        fallbackCopy(color);
    }

    function fallbackCopy(color) {
        var input = document.createElement('textarea');
        input.value = color;
        input.setAttribute('readonly', '');
        input.style.position = 'fixed';
        input.style.opacity = '0';
        document.body.appendChild(input);
        input.select();
        document.execCommand('copy');
        input.remove();
        showToast('已复制 ' + color);
    }

    function openTheme(theme) {
        dialogArt.className = 'dialog-art';
        dialogArt.style.cssText = colorVariables(theme.colors);
        dialogArt.innerHTML = symbolMarkup(theme);
        document.getElementById('dialog-theme-number').textContent = 'ARCHIVE NO. ' + theme.id + ' / ' + seriesNames[theme.series];
        document.getElementById('dialog-theme-name').textContent = theme.name;
        document.getElementById('dialog-theme-en').textContent = theme.en;
        document.getElementById('dialog-theme-description').textContent = theme.description;
        document.getElementById('dialog-palette').innerHTML = theme.colors.map(function (color, index) {
            return '<button type="button" data-color="' + color + '" style="--swatch:' + color + '"><span>0' + (index + 1) + '</span><strong>' + color + '</strong></button>';
        }).join('');
        var applyButton = document.getElementById('dialog-apply');
        var themeKey = implementedThemes[theme.id];
        applyButton.hidden = !themeKey;
        applyButton.dataset.themeKey = themeKey || '';
        dialog.showModal();
    }

    filterRoot.addEventListener('click', function (event) {
        var button = event.target.closest('[data-series]');
        if (!button) return;
        activeSeries = button.dataset.series;
        renderFilters();
        renderGallery();
    });

    searchInput.addEventListener('input', renderGallery);

    gallery.addEventListener('click', function (event) {
        var card = event.target.closest('[data-theme-id]');
        if (!card) return;
        openTheme(themes.find(function (theme) { return theme.id === card.dataset.themeId; }));
    });

    gallery.addEventListener('keydown', function (event) {
        if (event.key !== 'Enter' && event.key !== ' ') return;
        var card = event.target.closest('[data-theme-id]');
        if (!card) return;
        event.preventDefault();
        openTheme(themes.find(function (theme) { return theme.id === card.dataset.themeId; }));
    });

    document.getElementById('dialog-close').addEventListener('click', function () { dialog.close(); });
    document.getElementById('dialog-palette').addEventListener('click', function (event) {
        var button = event.target.closest('[data-color]');
        if (button) copyColor(button.dataset.color);
    });
    dialog.addEventListener('click', function (event) {
        if (event.target === dialog) dialog.close();
    });
    document.addEventListener('keydown', function (event) {
        if (event.key === '/' && document.activeElement !== searchInput && !dialog.open) {
            event.preventDefault();
            searchInput.focus();
        }
    });

    document.querySelectorAll('[data-hero-theme]').forEach(function (node) {
        var theme = themes.find(function (item) { return item.id === node.dataset.heroTheme; });
        if (theme) node.innerHTML = symbolMarkup(theme);
    });

    document.getElementById('dialog-apply').addEventListener('click', function () {
        var key = this.dataset.themeKey;
        if (!key) return;
        try {
            window.localStorage.setItem('guccang-theme', key);
        } catch (error) {
            // 隐私模式下仅当前页面临时生效。
        }
        document.documentElement.dataset.theme = key;
        showToast('已应用主题，其他页面将同步生效');
    });

    renderFilters();
    renderGallery();
})();
