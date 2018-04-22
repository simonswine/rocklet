package navmap

import (
	"bytes"
	"compress/gzip"
	"encoding/hex"
	"os"
	"testing"

	"github.com/rs/zerolog"
)

var exmapleDatabase = "1F8B0800000000000003ED9D077C53C51FC013868208C89455966CE18FC8109051649436797979B9DCBD3D93A6CD0E201B448620A3B2558688A8ADC806111922141007A2882802FA77306429537040FFCF02D24DFBA750D2DCB79F6B93974BFA7EDF7777EFEEDE48FFFE550D4D58A3C1683018BED453504F250C350CCFF4301A1AEB0B1D7AEAAEA712FF64C06030180C0683C16030180C0683C16030180C0683C16030180C0683C16030180C0683C16030180C0683C16030180C0683C16030180C0683C16030180CA68830E64451AFD45DC598969D885260CCD940E428D0434D5790D543C428D04B40A41BC8A10A4494821C1B818852908780C83090B508647E1E010AB2D5812C0B8ABF02A3215BDC91A520876630C20A415ECD604428C86B4F78534151AFE51DC498A52B98B38FF02D04B71EEDE6D740B82AC8BAA3CF9E215F95E09A83B09C34B885817FC2C99F808C144924FF2FD90C64AE146979F50272093FCC4A417603D737E2BF4B0A66203DB7C110460AB21948BB6EE0C62BC6FC6DF90C0AD2CB518156A248AB4DB621CFF5B62CC3A6CF6F5398317B410C1471B391F3A02FDF3B80DC35E4B7221479B399E7B0F7360CE4AF2928F2F00D590DE4B5ED0BD61AE4C7C0BD107F5603798C01B2ED196F61E0964DC1BD117F010CDCD4902F03B7680CEF95F00D791930DEDC1D169CBC6BC13D147F5EEDC01D33704FC55F9096B06006728BFFDE1B34DC5D03F75CF886BB6BE05E8CFF4E19C8616070EF15FFEB14BE819C6649D2728EFF9E9072270C64FD1FD7C2CF1CEE3D117C3A77DEC0B5D986AC0B72C85344148E81EB7369E9C53DB381F485371EFFF3ECDF58D3D29F177DF3505806720FE45A15B8468687F740ECD728B4329007FF049B969679C6F56EC5776B723450600DB97701B3CE34DFD5E0F2453603C6C2339079D3DF43CD7F2672345070F2AA05F768E437C87614BCF00DDCE31496819B8796C24D462119C8F819451D5201295403C6EB9F11566003855B0B8C116120C361E55C5E2FEA980A44F6B3E26E1E2ECC7D3B677410EE575DE461201F18331A08A723E619C962205FA7CE657D4B7893BB817CF68FC3ACD067E7F6CB0036504C0DE47F90580C0D18AF8DE90B70A03CBCB9ED5A800D44A0817F7BC5378EAE177508B7C9FF5506320D0C8A9981DC7AC5597AFE69E9139FE1380CC84ECEE794E6A020D3A18E2CAF843385719CB0A863B83DB081FC18B8D51976451DC3ED913F03D71BFE0CEF0ADB09916CE4CB402EF9AE7F4291AE7F2190A70163FA8CD835059962BFF673ED038A74F50B81BC0CFC7B954DD68E92C1101106FEEDF5A54799E5ECDB7BF7507081C96BD23BC3263764EA11A5DD8BE701FCBFE46220F3048931BB81E2E320D76140C680AFD5F56CB9FE59563CDB81F47D7CA605FFE4CC2163313570A38F930F0337CB471893CDC08D3E5E8625D7DA81083160CC418031FBDEB0181BB8B134E3A1A31CFA03C5D7C08D706E5C807DF36840E41948FF9D61DC1769066E952F220DE4CE5D5CDB3B41C10D18C2F65C899CC9AF81E20B36800D6003D80036909381C8F290DD4064C59FC31D18224D404E734445BD4A77196C001BC86620C25BC2B462708E6841C96EA0A8D7E86E830D6003D840C41BC8E1AEF445BD4A771B6C20B7E3869143F63B3014F51ADD6522EC9B187222E267C9B27E174551AF4F11800DE07620DB3526110736800D6003D80036800D6003D80036800D6003D800368047C7B80CE032800D643B5E90E7A504395C5210F6971A643F6A96F10B7DD2722663A859AECE0D3FF2BCFF40AED71866B933CF4D0361E8E0F6EFC492D94C51C753700AD7002E03B80C84A3026CA0700D846397AA90CB40183685856AE07A4FA1A8632A18856CE09A86A20EAA4014BA01E3F56FB62AEAC0F24DE11B08B7A6E08ED482B0B270270CA4855325C87B749C4B7469B7BEA3751895826C06322ECA34FA355EFF959F2FBF0EA77D624E0632CDF9A4DF8EC598C1407ABA45ED3186D1E54A3918C8F0E2B55FC6F4D30E331948BBB12017036174E96A9E0632869196BD3AE45506C2A72DCCDB407A8EAC398D37B6729E65206CB8B5814C39D36BC1F52FEFCDAB29082305F9377083B47FEF521B4906723B6E9043E86177E4A0E065208F3787D7C6BF0E36707B06F23EA0161E64BBD2EAB63EACB0D6EA6E52A806C2922CED79786EC6DB039F4482AFB5CAE1DB6C3237EA59468B61DAE2E745B6B6F05A8370F3F52CF98B5F49C979BA27B7BC4663F1339053CF2ED7ACC662B9D32CC02D898CB7CA109E64AF069166A00077E2C00622D38031879CC5CD40EE3723B9F120EB9C500418C836DACF6735095372D819640BB5788F1EF233F18F0D6003C5D6C0B563A3116E20F7F023C280E1C6BE3E920DFC0336906B5310A9066E768223D6C0BF4B23DE406E07CA8BBB810C43C35B350FC5875C0C14DBAFF4CB8E31C7F872ED2C6203D840A418C8FDE4E1626EE0D6BB82C83190EBB8B19819C8D6E1B931459AFBBC41713790766364103106720D34E7E823DE002E03D84071349077AB9735F86268201D6C001B28586350AC0DE4AB4528E6066E41B13770AB8DFFCFEF6279067A81DA81627642E535B0817CB57FD7270C8ADBB9E5D7C065001BC0060A66A0A8D7F64E800D6003F9DB1BDEF85BD46B7B27284819289602702DC8CF6D9A3350D46B8BC16030180C0683C16030180C0683C16030180C0683C16030118FD15046FF31182AC61B0C7F3B0CE9CFFE794E3306C33193C150D250D510FDB0C1F0456983A1D48D37390DCE8ACE36CEDF1C4F38673B44E76087EC2CE530397B6A26E76835D6B94B79CAB948F9CE3153D9E8E014BBE39CDCDC7144BEA44D90DFD42E49AF6973A43D9A4B5AAB9D113768C9E2314D138F6A55C4B28E1F848B5AA270542B29D4712CE4AB3858BE86A3027F4E9BC71DD054EE84F6397B589BC45672706C39C70AA6BA630653CD6161F66A87E9155A349DAA7D859A3A96A1C58E4DC8E5DC8C6AC76F464BE397A0BDF1EFA3BEAE0F91DB351BF95D2FA3E9AEED688DEB006A9EF0302D2458E845096FD33B123EA6B7267C467B12CFD275DD5666A8FB0B66BA7B0C3BD8BD8D75BBDF613F76AF611FF428EC49F7026697BB3673DABD9E069E2FE8573C6BE8F25E03DDD83B075DF68C413B3CC3517BCF00F48EBB310AB84B20D5DD08B575B7458B12FF827F259C8407137E870F249A903771398A4BDC833E4B5888CA278C47A35CEFA15EAE0F507FD75E542BA1111D9FF0183D3C611ABD316137FD69C21E3A36F14B7A78620ADD3BB13EBD36613EAA972020972B843E8D7F01CD8C1F87CAC57B11E9E4D00407873ED43874BFC62342E5D1348547DFC822AA220B0848029A2BF268AFC0A19A82849EE637A387F9DF50257E2F6AC8FB507F7E247A978F471E61042A27CE42BDC449E805712232493350055977278F42CBE47E485156A06F95C7E98755826EAEEEA0CDAAC234555F662E292DD9BF14177B5571B2B5D479EC30B50C37559538A06EE02AAA67B83F9464DEA0FA04A3DA424C538E8B5794FF4857945ED213EA3CA99FBA498A579F961D6A35C5A17EAB706A3FB59BBA4ABDACDCAFFDAACCD3CE29EF6B679535DAEBEA6A0D69E51C9F6ABCE3A836D451CDF1A563BCA383F30D4727E711477B673DE7364713E79BFAB3998E0ACEB7B512CED36A696740FDC491A64C756C51273A3ED0C63BCA3B9E73947000C779AD9E23468B7200B59EE39CF2906388F29986948DDA653D5D50DFD7446DAD36480B6A5DB51E9AAC466BBD94AE5A13B9A75641EAA39D12FA68BBF9186D3917A34D6563B4A14C1FED59BAB7F6137A4A7B1BD5D4B6A11FD41EF4215560FEABF6670FA8D3B9FDEA0A7EBFBA57F8463D257EADDE2FEF531B287BD42EEA2EF54F75B75A435BAFF6D0147586EA549BE9E923C5A9262A2EB58C92A82E96BD2A2107D493BAA781D240758538449D260C5607F183D5527AEACC2D50157689BA8449513B339FA9D3698776167DAE4D42710E1F7AC4E9414FC50F44355C63D101D7363422A13A9D9AD0973E9AA0D0251347D2BEC4D1746A62097A7DE24A743071236AEAAE47B7718FA25BBB27D0C3DD53E80FDC67D15AF701F4B37B17AAE5F90355F20CA52B7A10FDACC7462FF350F45ECF18FA47CF4EFA9C5E17FEEBD9457FEB49A165EF24BA8E6F00BDD7B792EEE68F62C6F99399897E965DEFFF9DDDE11FC98DF0CFE52EF856712D7CDBB81DDEE9FCE75E42F8C49B2A8CF09E174A794F0856CF4F42234F5531CAB35BACEA692D95F73497BEF13492C67BEB4B1D7C75A403BE1AD2087F6DA956204A8A0E54905A06D68A0F05BA8B3D0295455FE0982007B608A302070425F08AE00AC8C2B840B4F07DC0233C149C24540F0E14CA0405E16AE049E187406B614DA0BD3035D053181C08094C609A400486094F061A0B64A0ACD03FB0857F3990CA1F0AA4F02702737922B89A7F2DF81B7F216816DA87FA0BD1A169823BF486303C1410FA85DA0ADB42ED854BA1ABFCF99087BF144AE14E844EB097438FB167435EE6B7D0DBF4F9D03174343407FD124A4175436F213EB814CD09AC4047FDCFA089FE01A8A13F11BDED53D1289F8676FB46A258DF30B4D8EB432E6F02EAE8F5A321DEA751AA773C7AD63B196DF14C4082E71C7AC1B3884EF5ECA3B77BD6D325BC0BE8315E0B5DDFDB801EEEAD48AFF3D6A0DBF96AD3AB7C35E9C7FD55E945FE23E8B49FA6FFEB8F657EF47FA7A773EC057F1A5732308C2F1798C6FFE8DFCBFDE07F9A3BEC5FC5A5F821BFC81F1416FACDE26BFE65E242BF510AF8CF8BE5FD03C4C33E5A38E00BF2FB7C7F725B7C3F702E5F7BFE115F05A1BAAF9EF8906FBA58C9B746DCEADD258EF72E1307791F1307787B88473D23C56F3D1BC4AF3D7BC58067B538CB13250ADE11426BDF21FE4F5F34BFDBFF005F35F013D737B09CEB1D20B89E81D36C74A022DB25309E7922F018D321709CAE1C8074D3C0AFA85FE01C5A1AF805950D3E428F09CEA6DF0FD662B6066DCC8BC1614C42702573393089F1049A32E5025FD269FE4974693D7FAFC0E7E8DDC0D7C8113C8816074FA2DAA15F9137B49C16429B1939B4875543A73931F4124F8712F85F824FF26F062F738B8273B8D7831DB93783ABD9946055F6ADE0556672F00F665570103B39F816DB2F78820D0463B860702B170836E73DC16E7C83E032AE4CF0B89E6AF325835D794390D3532DFE62A0323F2F70886B1598CC7DEF6FCDADF057E192FD93B969FE0FB9B6BAEF2DBE69DC02DF2C6E9C6F0377C2BB95DBE53DC06DF13EC7EFF6F612BEF07E248CF25612F77AAE0A233C0705D6F3BD70DAFDBD60761F125AB9078B5DDC27C5EF132B4A8989A5A54B09A5A543093DA5D484765230B195F498BBA5F493BB8534D4B34F643CD545D9F38DA078F60BCD3C87845DEE28719F1B883FBAA1C87B6CE269CF15C1E59D275CF21EE6833E81DFEFFB82B3F91DDC12FF0EF6A4FF3EF688BF0F73D23F8DDEEF2F4BBFEFDF8952FCF3D012BDECBEE4EF8380DF87CEFB02A8B35E6E877A9F43EF7A26A2769E3F91C5D3835EE4B1D28217D0957C80DEEEB3D3C3FC50DF9635F56DB911750DCC40AEC05AD43A908466F83944F9E3D1A3FE37F5F27F16A9BE727467DF8374695F353AC6FB93BE7F4D46C33C01C47982A887C78DF6B86BA2B1EEBFE140776544BA9F42D5DC3DD16B897FC3DF12D6C36E0963E0785717F8597C753826FE69F87EFC08C8C66BB05B7C15E889AF05A9F87A709CB3015CE778049ED71AC1965A63E8579BC0154A5378416E0A3BCB2DE020A9057C476C062F0B4D605BA1310CF08DE105EE13F80737011DE32EA013DC2FE8017E0532F216D48E6F85E6F132BAC207510B2184BE155E40EDC517D0527D3F9AA8EF439BEAFB504E1E8E36CB039147D988BE5436A353CA7AB4587D17F9B475A8B5E33DF4AB63235AAAF75DECF11FA0E7E293509ADE7FD15C4BD079D7DB6856C26BA873E25C7424F10DF48AFB79BD1D1E848EB869F498BB37EA9ED80D5D4C08A299090ED4294146275D4FA3B9AE1834D3751E4E736D80CD5D83F5F867C3E3F13C3C1FDF0C1E8EEFAEFF1D02FBBA26C00AAEDDF042FC5158D2D5069D8C8FD6FB4CFDD1A67813EA195F1A1D762E820B9CDDE033CE87E170670D58DE59176A8E9570A3A31DAAEC34A106CE95B0B6938546270F373B36C2B71C044A76CC41F18E7EA88AE3342CEBE8078D8EDE708D3616F6D7AAA141DA5894A8CD440F69DDD16135051E53CD70B3CA4345DD05FBAA3664569F401F295168933205A62ABD6092E2871D9533B08D5213B5525AA115722A4C9105385D86B0B7BC0C3691DBA066B20F1D913AA2F9D22A3847027084648775A46DF08AD8075D1207A0D5627B344C4C8683C5A9D0213E03AB88CFC3D9C2545842488213F8A9B02BFF20ACC4D780D3B93A701F1B05ABB1F520C5D48349747DF831AA0FCBE9C902EBC199F628B80744C14AA00EA46C0FC3D9D4C3F02B6B4D58D9DA089E225BC210590746912DE0484B1BB889680D2F9A1F83ADCDADE04ED340B8C3F4285A6E9A82DE328D47674DE3D0BBE667D170E219D4DA32129DB68C40EF9143D100EBD3A80B15426954006DB54D4314284327032BBD11307405BB9BFECA1EA29BC1DE743758977E1CBE859C5042272144B39188206D470D191EFDCCCC4789ECDF6821FB207D88AD4A5F657F4793B9A3E8303707B5E4EF436DF858D88EAF08DBF3D5E109AE069CC9F5844BB8853095E3D007DCB37A7A1EB1DC4118CFF920C3D1F041FDB583AC82BE6557A17D6C17F40C5B1A99D9399062C7C34F98D19064F4ED444F853DE924DDD564D8574FEFC3C9B00FAC800838049583DFA06FECC7E101BB07EEB57786A3ED2360577B7BD4CD7E0875B5AF4355EC71680378182583713005C4402718A1FB2D81EE07F55009D0024DB1AD80493605F6B3C5C3476D1FC26AB6EAA88AAD0EDA4235412F50C7611235004EA420EC49BD00AB5155512DAA057AD13A19BE680530DE3A0036B69E8251D6EAA8BEB5055A4A6E87F3490D4E2039F824B916D6246BA1DA640374DCD214BD66B90A975A86C1372C76E8B08C872D2D15510B4B6BB498580D57103C9C4970D04CAC87ED8828D48E68827E37B7466F9A2BA0B7CCE3E062330943E601B0B3F934EC60AE8DDA991BA0BF4C2E74D23411FD683A0A2F99BAC3ABA61A30CDD4080E353583EBE29AC17E71CBE0E4381E4D8F9B8A66C48D458D4CCFA2A3A62864D0DFEB3745A3CAA613B08A6906AC669A0879D32468354F8001E27938DD3211AE2327C383D6297011350BF6B5FD0D0FD986220FD88F3682F2F40610452F043CFD16B848AF002AD3074C663EB08D66ACB6D7998BD444660895C0582885694EB998F15627934A4A0C452E637C6473B60E6965E75BAAB2352C49CCD74457E610D188F98A68C8988916CCABE656CC39530BA693B92DF3A7A921D3D3DC81A96A7E82D96F72E96933B3DB94A23F5FC5BC6BAEC6362156B05D8846DC61732F6EBD79006724FEE61A12C3F95FCCDBF978F3277CB47935FF8CF907FEA4D92C9427960BDF9A170AAF990F08E3CDD1E2A7E64DE206731789325BA41DA609D226D353F272934179C5B455996BAAACBE626AA96E37F550BF331D55CA9A53959AE6614AC8FCBCF2B3F91DA512F197028991EA10E255B53F21ABFB08561D6D09A9D1E44BEA6E72B76AB06E54AB5967ABE5AC75D5FBACEB944ED6379587ACB1CA4EF267F92B72937C902C25AF24174A9BC99ED24AF263F16BF225F157F249B193B5A5B8CEDA4FEC405D16FD5448E2A95D524BEA8414453D28B7A5526433B555F6511D95D7A9F9CA2C6A8A7286F22AA7A818658FBEFC53CA27FF41B5951FB61D93CE531BA553D47CE967EA3FD24CEA7531897A5CFC85AA2E0EB251E2AFB6AFC547C1FDD26360AE34007492C7826F6504DA2808CC551E012F29CF034949014F2A25EC82D2C6DE45D90A1628D380A21C0436E5713BA554B673CA16D04349056E651E18A3BC02362BE3C13CE5653050590D062B53C008C501024A1C98A6C480B7942E609262010B95762059A90D929427C0286510784EA1C068E525F0BCD2074C549AE8FFBB1258A69CB12D52EED71FD7D79775063E6502909515FAFA0C04F594B1E0843C116C9167811DF20070467E42FF8BC0DF72397044AE09FE904B83EFE51A6097DC02BC2777078BE4B660AA1C0352E4D6E0A05C467F5F53B0406E099AC966D0416E036AC971C028CBA0A2DC0774D1F327CA1DC10CFD3DEFE8EF37282D419A4C812F6427F88F1C02072537D82A2582FDD26090268D0395E564DDDBC7A0AFBC1644CB33C162F935B05EF738527681EE320134D9AEBF1603149901B172579020436096A3410F990335F4FF6FD0979D911E04E7A5EEFAE7350317252FF8419A068E499F8095D220F0AAF432182E5D05E3A586F6E7A42B4095CAD81DD211304E1A0F5A497D4007E915D04C7A165C12CBD9ABE8792E88DF8379D2545056FE087C250D03CBA536FAE7F4030BA496608D6400DBA44DB6C512D29F8F0249D2DB6084F41918261D03A3F5CF9DA1BF7FA4740928526BBB242D0315A54D60A7781AA48865ECC9E223F6F9A2D9FEACD8CBEED0534931D67E54E861DF265C00978515C02036045F0BA76C0B85E76C2D85806D0FFF8E6D099F6A7B8EBF622BC7B70066BE1C98C3FF65F3F025412DFE01B0926B0C26722218C50D059DB868F02E1B033AB073C1BBCC03F61ACC60FB48FAB4BD95BEBFE94827D83BEAEDCB13F47C309D5E0696D1AB403D661430333C58C87407BF306D01C3D60731DCCFB6C9DCE7B6DEDC7BB61EEC07B69F992DB6FD4C92ED23A68BED3D6690ED1273C0A6B1A76D1BD9E5B64FD929B607389BAD13F7115596EB464D60FFB25E656A52AB99717AAA64BBC28CB045B3536D17D9D76C8BB8EDB65FB975B6527C9CED3E3ECAE6D45353A19E6D9D9EF60AC7A82BC2748A10A651EDF8D9D41E6E363556FF3C81AB4495E72B510D856AD4AB42256AA770D49A2CACB2961552ACA7B937AC3FB36F5A3BB0C9D6834C929EA2ADDDD896D63E5C1BAB836F6EA58446D6D54235AB475C4EAE14279323C549A45F9848027E22D9899B4C6E629348554FB3D84164158E271339488EE0113957B091EF8914B943B293FB6444AE5168B2AEDA8D6CACFE68F954D96C39A2ECB49454575908B5B745541B5A5875A945537DE41BEA77E4676A1AF9A55ACB4A6994D5A5B9AC497A5AA53F9EA901EB40ADA3B5AED6D3FA9B6AB196D0DA59B7A967495A3D4836572F92BB944AD69794B36453456FC3F47454DE4C2E96D793CDE4247282349D94A464B2AEB49A5C206E27BB8B6DACBDC48DD625624BAA9DD894FA4ED86F2D2FDAAD63C52DE4EFA24AAE9500D943EE4D3E23D722E7C865C9414A59729F721FF9B6528A5C2E9726A74B25C890781F490865C9767C59F271AE2CB9937D806CCFA55A0E71DB2D97F8ED968A62AAE55169BB658CB4C3F289F481A5AAECB594922F126F4A6B880ED2974419A99DA593F4AA6588B4C932521A6CA92335B47C279E225A8A6788B2C225E224F707F13BFB07F1929EAEB2B4A52B77CC427029A48353AD23B97A543FEE0055951B6D1BC27E6F2BCF1EB01D6296D87630CFDB563209B69799EAB66446A56632B3ADE3983FC9D94C12F911E3D6F76DF5C869EC8B1685BDCF528DDD469C61D613F7B3C709899D6451D9AAE452B60D5999DB6A91B9A696386E0FF118B79AD8C02E2782EC32622DBB86F8885B459CE1DF212A882B8917C4A5C43CF11942145B109F098F12738516C483E2234463A909F194DC94382C37211A2816E233D944AC944CC44CD1420C162CC43C9E221278481CE3BB13A5C54A444DE947F348E973B32AEF3497553E328BCA347337A58339556E623E2E3536EFD053AA74D5C4492F9BBE14AB990E8B57E3EA4B73E31A4A43E3164AA6B863D291D873D2A658281F8A3D20BF1CDB50A91E7B457EBEEF09D9DDF7805CB2EF103925A6A33C33A6A9BC24669D9C1CF388F26ACC70A542DF5F94A57D29D5123B569D15DB477D23B6B47A35B696DA3F6E98BA328E57B7C4D552E7C52D57DE890B29554D3EA5B3C9A9BC6F1AA29C353DAA9C3125C8674C29D219536BE99CA997F4A2E98CD8D8F4B958DDF4937839AEA6B4292E51EA6A9A228D31FDA1A78DF228D3EFB2CB5457A967AA2B3730CD901E31ED91D6C7FD2E2D8FFB585E1DD74479236EA2628F5BA0F48C4B95A3E368B95BDC62B9545C036543EC106549EC1BCAD25887523EEE7EE5FDB889F25A535939D9BC52B212DDA42A9624F1B4658BB082FC951F6C8DE6EB53ABB865D47FB876B615EC161BA597890AEC2ADB4166BC6D15F30BF51EC351CB98246A3173805ACEFC45A5304BA817994FAD539826D6494C496B32B3935CC9CC268D2C4F3AD95F2D6E96B554600F1373199618C22C334BCC13E65E4C19731453DEDC98D9636AC63C6B42CCEBA621CC68D35A669BE934F3B9E90BE694692533C8BC9B4935F7617698AB321F9A1B3093F5FC474C759992E6860C696ECE34379B99632643A93286AA0683E107EA7893F3E5E35FFFBAF1975B6A2DEBFF6894D2B4FFFF004E35EF663A590100"

func newNavMap() *NavMap {
	return &NavMap{
		logger: zerolog.New(os.Stdout).Level(zerolog.DebugLevel),
	}
}

func TestListCleaning(t *testing.T) {
	n := newNavMap()

	cleanings, err := n.ListCleanings("./robot.db")
	if err != nil {
		t.Error("unexpected error", err)
	}

	if len(cleanings) < 1 {
		t.Error("no cleanings founds")
	}
}

func TestConvertMap(t *testing.T) {
	n := newNavMap()

	data, err := hex.DecodeString(exmapleDatabase)
	if err != nil {
		t.Error("unexpected error", err)
	}

	r, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Error("unexpected error", err)
	}

	if err := n.convertMap(r); err != nil {
		t.Error("unexpected error", err)
	}
}
